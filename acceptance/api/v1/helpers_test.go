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

const (
	// The filer is addressed by workload so the test does not depend on the
	// generated pod name. Both values follow from the chart's
	// `seaweedfs.fullnameOverride`.
	s3FilerPod       = "statefulset/seaweedfs-filer"
	s3FilerBucketURL = "http://localhost:8888/buckets/epinio/"
)

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

// listS3Blobs returns the names of all objects currently stored on S3.
//
// SeaweedFS' filer serves the bucket contents over plain HTTP inside the
// cluster, so the tests read it directly: no S3 client, no credentials, and
// no helper pod to pull and wait for.
func listS3Blobs() []string {
	out, err := proc.Kubectl("exec", "-n", "epinio", s3FilerPod, "--",
		"curl", "-s", "-H", "Accept: application/json",
		s3FilerBucketURL+"?limit=1000")
	Expect(err).ToNot(HaveOccurred(), out)

	// The filer answers 404 with an empty body until the bucket exists.
	start := strings.Index(out, "{")
	if start < 0 {
		return nil
	}

	listing := struct {
		Entries []struct {
			FullPath string
		}
	}{}

	err = json.Unmarshal([]byte(out[start:]), &listing)
	Expect(err).ToNot(HaveOccurred(), out)

	bucketPath := "/buckets/epinio/"

	blobs := make([]string, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		blobs = append(blobs,
			strings.TrimPrefix(entry.FullPath, bucketPath))
	}

	return blobs
}
