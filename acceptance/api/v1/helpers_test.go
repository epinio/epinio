// Copyright © 2021 - 2023 SUSE LLC
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
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"

	. "github.com/onsi/gomega"
)

const minioHelperPod = "miniocli"

func uploadRequest(url, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open tarball")
	}
	defer file.Close()

	// create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create multiform part")
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write to multiform part")
	}

	err = writer.Close()
	if err != nil {
		return nil, errors.Wrap(err, "failed to close multiform")
	}

	// make the request
	request, err := http.NewRequest("POST", url, body)
	request.SetBasicAuth(env.EpinioUser, env.EpinioPassword)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build request")
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())

	return request, nil
}

func uploadApplication(appName, namespace string) *models.UploadResponse {
	uploadURL := serverURL + v1.Root + "/" + v1.Routes.Path("AppUpload", namespace, appName)
	uploadPath := testenv.TestAssetPath("sample-app.tar")
	uploadRequest, err := uploadRequest(uploadURL, uploadPath)
	Expect(err).ToNot(HaveOccurred())
	resp, err := env.Client().Do(uploadRequest)
	Expect(err).ToNot(HaveOccurred())
	bodyBytes, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())

	respObj := &models.UploadResponse{}
	err = json.Unmarshal(bodyBytes, &respObj)
	Expect(err).ToNot(HaveOccurred())

	return respObj
}

func stageApplication(appName, namespace string, stageRequest models.StageRequest) *models.StageResponse {
	b, err := json.Marshal(stageRequest)
	Expect(err).NotTo(HaveOccurred())
	body := string(b)

	url := serverURL + v1.Root + "/" + v1.Routes.Path("AppStage", namespace, appName)
	response, err := env.Curl("POST", url, strings.NewReader(body))
	Expect(err).NotTo(HaveOccurred())

	b, err = io.ReadAll(response.Body)
	Expect(err).NotTo(HaveOccurred())
	Expect(response.StatusCode).To(Equal(200), string(b))

	stage := &models.StageResponse{}
	err = json.Unmarshal(b, stage)
	Expect(err).NotTo(HaveOccurred())

	jobName := names.GenerateResourceName("stage", namespace, appName, stage.Stage.ID)
	waitForStaging(jobName)
	return stage
}

func waitForStaging(jobName string) {
	Eventually(func() string {
		out, err := proc.Kubectl("get", "job",
			"--namespace", testenv.Namespace,
			jobName,
			"-o", "jsonpath={.status.conditions[0].status}")
		Expect(err).NotTo(HaveOccurred(), out)
		return out
	}, "5m").Should(Equal("True"))
}

// Create the S3 helper pod if it doesn't exist yet
func createS3HelperPod() {
	out, err := proc.Kubectl("get", "pod", "-o", "name", minioHelperPod)
	if err != nil {
		// Only fail if the error isn't about the pod missing
		Expect(out).To(MatchRegexp("not found"))
	}
	if strings.TrimSpace(out) == "pod/"+minioHelperPod { // already exists
		return
	}

	out, err = proc.Kubectl("get", "secret",
		"-n", "epinio",
		"minio-creds", "-o", "jsonpath={.data.accesskey}")
	Expect(err).ToNot(HaveOccurred(), out)
	accessKey, err := base64.StdEncoding.DecodeString(string(out))
	Expect(err).ToNot(HaveOccurred(), string(out))

	out, err = proc.Kubectl("get", "secret",
		"-n", "epinio",
		"minio-creds", "-o", "jsonpath={.data.secretkey}")
	Expect(err).ToNot(HaveOccurred(), out)
	secretKey, err := base64.StdEncoding.DecodeString(string(out))
	Expect(err).ToNot(HaveOccurred(), string(out))

	// Start the pod
	// FIX: pinning the minio CLI while waiting for this fix https://github.com/minio/mc/issues/4024
	out, err = proc.Kubectl("run", minioHelperPod, "--image=minio/mc:RELEASE.2022-03-13T22-34-00Z", "--command", "--", "/bin/bash", "-c", "trap : TERM INT; sleep infinity & wait")
	Expect(err).ToNot(HaveOccurred(), out)

	// Wait until the pod is ready
	out, err = proc.Kubectl("wait", "--for=condition=ready", "pod", minioHelperPod)
	Expect(err).ToNot(HaveOccurred(), out)

	// Setup "mc" to talk to our minio endpoint (the "mc alias" command)
	out, err = proc.Kubectl("exec", minioHelperPod, "--", "mc", "--insecure", "alias", "set", "minio",
		"https://minio.epinio.svc.cluster.local:9000", string(accessKey), string(secretKey))
	Expect(err).ToNot(HaveOccurred(), out)
}

// returns all the objects currently stored on the S3 storage
func listS3Blobs() []string {
	// list all objects in the bucket (the "mc --quiet ls" command)
	out, err := proc.Kubectl("exec", minioHelperPod, "--", "mc", "--insecure", "--quiet", "ls", "minio/epinio")
	Expect(err).ToNot(HaveOccurred(), out)

	return strings.Split(string(out), "\n")
}
