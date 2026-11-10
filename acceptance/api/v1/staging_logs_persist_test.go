// Copyright © 2021 - 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Staging logs persistence", LApplication, func() {
	var (
		namespace string
		appName   string
	)

	BeforeEach(func() {
		if !stagingLogsPersistEnabled() {
			Skip("server.stagingWorkloads.persistLogs is disabled")
		}

		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()

		By("creating application resource first")
		appCreateRequest := models.ApplicationCreateRequest{Name: appName}
		bodyBytes, statusCode := appCreate(namespace, toJSON(appCreateRequest))
		Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	It("round-trips write/read/delete of persisted staging logs", func() {
		By("uploading and staging the application")
		uploadResponse := uploadApplication(appName, namespace)
		stageRequest := models.StageRequest{
			App: models.AppRef{
				Meta: models.Meta{
					Name:      appName,
					Namespace: namespace,
				},
			},
			BlobUID: uploadResponse.BlobUID,
		}
		stageResponse := stageApplication(appName, namespace, stageRequest)
		stageID := stageResponse.Stage.ID
		Expect(stageID).ToNot(BeEmpty())

		objectKey := fmt.Sprintf("staging-logs/%s/%s.ndjson", namespace, stageID)
		pointerKey := fmt.Sprintf("staging-logs/%s/apps/%s", namespace, appName)

		By("waiting for the server-owned persist to write the staging logs")
		Eventually(func() bool { return s3ObjectExists(objectKey) }, "2m", "2s").Should(BeTrue())
		Eventually(func() bool { return s3ObjectExists(pointerKey) }, "2m", "2s").Should(BeTrue())

		By("removing the staging job so the stored-logs path is used")
		out, err := proc.Kubectl("delete", "job",
			"-n", testenv.Namespace,
			"-l", fmt.Sprintf("epinio.io/stage-id=%s", stageID),
			"--wait=true")
		Expect(err).ToNot(HaveOccurred(), out)

		By("reading the persisted staging logs")
		logs := readStagingLogs(namespace, stageID, false)
		Expect(logs).ToNot(BeEmpty())
		Expect(logs).To(Or(
			ContainSubstring("buildpack"),
			ContainSubstring("download-s3-blob"),
			ContainSubstring("unpack-blob"),
		))

		By("deleting the application which removes the stored logs")
		env.DeleteApp(appName)

		Eventually(func() bool { return s3ObjectExists(objectKey) }, "2m", "2s").Should(BeFalse())
		Eventually(func() bool { return s3ObjectExists(pointerKey) }, "2m", "2s").Should(BeFalse())

		By("confirming the stored logs are gone")
		token, err := authToken()
		Expect(err).ToNot(HaveOccurred())
		wsURL := fmt.Sprintf("%s%s/%s?follow=false",
			websocketURL, v1.WsRoot, v1.WsRoutes.Path("StagingLogs", namespace, stageID))
		_, err = env.MakeWebSocketConnection(token, wsURL)
		Expect(err).To(HaveOccurred())
	})
})

func s3ObjectExists(key string) bool {
	out, err := proc.Kubectl("exec", "-n", "epinio", s3FilerPod, "--",
		"curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
		s3FilerBucketURL+key)
	return err == nil && strings.TrimSpace(out) == "200"
}

func stagingLogsPersistEnabled() bool {
	out, err := proc.Kubectl("get", "configmap",
		"-n", testenv.Namespace,
		"epinio-stage-scripts",
		"-o", "jsonpath={.data.staging-values\\.json}")
	if err != nil {
		return false
	}

	var values struct {
		PersistLogs bool `json:"persistLogs"`
	}
	if err := json.Unmarshal([]byte(out), &values); err != nil {
		return false
	}
	return values.PersistLogs
}

func readStagingLogs(namespace, stageID string, follow bool) string {
	token, err := authToken()
	Expect(err).ToNot(HaveOccurred())

	wsURL := fmt.Sprintf("%s%s/%s?follow=%t",
		websocketURL, v1.WsRoot, v1.WsRoutes.Path("StagingLogs", namespace, stageID), follow)
	wsConn, err := env.MakeWebSocketConnection(token, wsURL)
	Expect(err).ToNot(HaveOccurred())

	var logs string
	Eventually(func() bool {
		_, message, err := wsConn.ReadMessage()
		if len(message) > 0 {
			logs = fmt.Sprintf("%s %s", logs, string(message))
		}
		return websocket.IsCloseError(err, websocket.CloseNormalClosure) ||
			(err != nil && strings.Contains(err.Error(), "close"))
	}, 120*time.Second, 1*time.Second).Should(BeTrue())

	_ = wsConn.Close()
	return logs
}
