// Copyright © 2021 - 2026 KRUMWARE LLC
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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	urlpkg "net/url"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Staging logs persistence", LApplication, Serial, func() {
	var (
		namespace string
		appName   string
	)

	BeforeEach(func() {
		restoreStagingValues := enableStagingLogsPersistence()
		DeferCleanup(restoreStagingValues)

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

		By("checking the server stored the real build output")
		stored := readStoredStagingLogs(objectKey)
		Expect(stored).ToNot(BeEmpty())
		for _, raw := range strings.Split(strings.TrimSpace(stored), "\n") {
			var line tailer.ContainerLogLine
			Expect(json.Unmarshal([]byte(raw), &line)).To(Succeed(), raw)
			Expect(line.PodName).ToNot(BeEmpty(), raw)
			Expect(line.ContainerName).ToNot(BeEmpty(), raw)
		}

		By("removing the staging job so the stored-logs path is used")
		out, err := proc.Kubectl("delete", "job",
			"-n", testenv.Namespace,
			"-l", fmt.Sprintf("epinio.io/stage-id=%s", stageID),
			"--wait=true")
		Expect(err).ToNot(HaveOccurred(), out)

		By("installing deterministic stored logs with a truncation notice")
		writeStoredStagingLogs(objectKey, []tailer.ContainerLogLine{
			{Message: "... 8 earlier log lines dropped, the stored logs are limited to 1048576 bytes ..."},
			{Message: "app-old", ContainerName: "app"},
			{Message: "app-new", ContainerName: "app"},
			{Message: "worker-new", ContainerName: "worker"},
		})

		By("replaying filtered stored logs while keeping follow=true open")
		token, err := authToken()
		Expect(err).ToNot(HaveOccurred())
		wsURL := fmt.Sprintf("%s%s/%s?follow=true&tail=1&include_containers=app",
			websocketURL, v1.WsRoot, v1.WsRoutes.Path("StagingLogs", namespace, stageID))
		wsConn, err := env.MakeWebSocketConnection(token, wsURL)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(wsConn.Close)

		Expect(readStoredStagingLogLine(wsConn).Message).To(ContainSubstring("8 earlier log lines dropped"))
		Expect(readStoredStagingLogLine(wsConn).Message).To(Equal("app-new"))

		By("applying a mid-session filter and receiving a complete marked replay")
		Expect(wsConn.WriteJSON(map[string]any{
			"type": "filter_params",
			"params": map[string]any{
				"tail":               1,
				"follow":             false,
				"include_containers": "worker",
			},
		})).To(Succeed())

		Expect(readStoredStagingLogLine(wsConn).Message).To(Equal("___FILTER_START___"))
		Expect(readStoredStagingLogLine(wsConn).Message).To(ContainSubstring("8 earlier log lines dropped"))
		Expect(readStoredStagingLogLine(wsConn).Message).To(Equal("worker-new"))
		Expect(readStoredStagingLogLine(wsConn).Message).To(Equal("___FILTER_COMPLETE___"))

		By("rejecting a bad mid-session filter without starting a UI refresh")
		Expect(wsConn.WriteJSON(map[string]any{
			"type": "filter_params",
			"params": map[string]any{
				"follow":             false,
				"include_containers": "[",
			},
		})).To(Succeed())
		Expect(wsConn.SetReadDeadline(time.Now().Add(time.Second))).To(Succeed())
		_, _, readErr := wsConn.ReadMessage()
		var timeoutErr net.Error
		Expect(errors.As(readErr, &timeoutErr)).To(BeTrue())
		Expect(timeoutErr.Timeout()).To(BeTrue())

		By("deleting the application which removes the stored logs")
		env.DeleteApp(appName)

		Eventually(func() bool { return s3ObjectExists(objectKey) }, "2m", "2s").Should(BeFalse())
		Eventually(func() bool { return s3ObjectExists(pointerKey) }, "2m", "2s").Should(BeFalse())

		By("confirming the stored logs endpoint returns 404")
		wsURL = fmt.Sprintf("%s%s/%s?follow=false",
			websocketURL, v1.WsRoot, v1.WsRoutes.Path("StagingLogs", namespace, stageID))
		Expect(stagingLogsHandshakeStatus(token, wsURL)).To(Equal(http.StatusNotFound))
	})
})

func s3ObjectExists(key string) bool {
	out, err := proc.Kubectl("exec", "-n", "epinio", s3FilerPod, "--",
		"curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
		s3FilerBucketURL+key)
	return err == nil && strings.TrimSpace(out) == "200"
}

func enableStagingLogsPersistence() func() {
	original, err := proc.Kubectl("get", "configmap",
		"-n", testenv.Namespace,
		"epinio-stage-scripts",
		"-o", "jsonpath={.data.staging-values\\.json}")
	Expect(err).ToNot(HaveOccurred(), original)

	var values map[string]any
	Expect(json.Unmarshal([]byte(original), &values)).To(Succeed())
	values["persistLogs"] = true

	updated, err := json.Marshal(values)
	Expect(err).ToNot(HaveOccurred())
	patchStagingValues(string(updated))

	return func() {
		patchStagingValues(original)
	}
}

func patchStagingValues(values string) {
	patch, err := json.Marshal(map[string]any{
		"data": map[string]string{
			"staging-values.json": values,
		},
	})
	Expect(err).ToNot(HaveOccurred())

	out, err := proc.Kubectl("patch", "configmap",
		"-n", testenv.Namespace,
		"epinio-stage-scripts",
		"--type=merge",
		"-p", string(patch))
	Expect(err).ToNot(HaveOccurred(), out)
}

func writeStoredStagingLogs(key string, lines []tailer.ContainerLogLine) {
	var body bytes.Buffer
	for _, line := range lines {
		Expect(json.NewEncoder(&body).Encode(line)).To(Succeed())
	}

	out, err := proc.Kubectl("exec", "-n", testenv.Namespace, s3FilerPod, "--",
		"curl", "-fsS", "-X", "PUT", "--data-binary", body.String(), s3FilerBucketURL+key)
	Expect(err).ToNot(HaveOccurred(), out)
}

func readStoredStagingLogs(key string) string {
	out, err := proc.Kubectl("exec", "-n", testenv.Namespace, s3FilerPod,
		"--", "curl", "-fsS", s3FilerBucketURL+key)
	Expect(err).ToNot(HaveOccurred(), out)
	return out
}

func readStoredStagingLogLine(conn *websocket.Conn) tailer.ContainerLogLine {
	Expect(conn.SetReadDeadline(time.Now().Add(30 * time.Second))).To(Succeed())
	var line tailer.ContainerLogLine
	Expect(conn.ReadJSON(&line)).To(Succeed())
	return line
}

func stagingLogsHandshakeStatus(token, wsURL string) int {
	u, err := urlpkg.Parse(wsURL)
	Expect(err).ToNot(HaveOccurred())
	query := u.Query()
	query.Set("authtoken", token)
	u.RawQuery = query.Encode()

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // acceptance environment uses a self-signed certificate
		},
	}
	conn, response, err := dialer.Dial(u.String(), nil)
	if conn != nil {
		_ = conn.Close()
	}
	Expect(err).To(HaveOccurred())
	Expect(response).ToNot(BeNil())
	defer func() {
		_ = response.Body.Close()
	}()
	return response.StatusCode
}
