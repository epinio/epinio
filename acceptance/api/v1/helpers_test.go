package v1_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/helmchart"
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
	bodyBytes, err := ioutil.ReadAll(resp.Body)
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

	b, err = ioutil.ReadAll(response.Body)
	Expect(err).NotTo(HaveOccurred())
	Expect(response.StatusCode).To(Equal(200), string(b))

	stage := &models.StageResponse{}
	err = json.Unmarshal(b, stage)
	Expect(err).NotTo(HaveOccurred())

	waitForPipeline(stage.Stage.ID)

	return stage
}

func waitForPipeline(stageID string) {
	Eventually(func() string {
		out, err := helpers.Kubectl("get", "pipelinerun",
			"--namespace", helmchart.TektonStagingNamespace,
			stageID,
			"-o", "jsonpath={.status.conditions[0].status}")
		Expect(err).NotTo(HaveOccurred())
		return out
	}, "5m").Should(Equal("True"))
}

// Create the S3 helper pod if it doesn't exist yet
func createS3HelperPod() {
	out, err := helpers.Kubectl("get", "pod", "-o", "name", minioHelperPod)
	if err != nil {
		// Only fail if the error isn't about the pod missing
		Expect(out).To(MatchRegexp("not found"))
	}
	if strings.TrimSpace(out) == "pod/"+minioHelperPod { // already exists
		return
	}

	out, err = helpers.Kubectl("get", "secret",
		"-n", "minio-epinio",
		"tenant-creds", "-o", "jsonpath={.data.accesskey}")
	Expect(err).ToNot(HaveOccurred(), out)
	accessKey, err := base64.StdEncoding.DecodeString(string(out))
	Expect(err).ToNot(HaveOccurred(), string(out))

	out, err = helpers.Kubectl("get", "secret",
		"-n", "minio-epinio",
		"tenant-creds", "-o", "jsonpath={.data.secretkey}")
	Expect(err).ToNot(HaveOccurred(), out)
	secretKey, err := base64.StdEncoding.DecodeString(string(out))
	Expect(err).ToNot(HaveOccurred(), string(out))

	// Start the pod
	out, err = helpers.Kubectl("run", minioHelperPod, "--image=minio/mc", "--command", "--", "sleep", "1000")
	Expect(err).ToNot(HaveOccurred(), out)

	// Wait until the pod is ready
	out, err = helpers.Kubectl("wait", "--for=condition=ready", "pod", minioHelperPod)
	Expect(err).ToNot(HaveOccurred(), out)

	// Setup "mc" to talk to our minio endpoint (the "mc alias" command)
	out, err = helpers.Kubectl("exec", minioHelperPod, "--", "mc", "alias", "set", "minio",
		"http://minio.minio-epinio.svc.cluster.local", string(accessKey), string(secretKey))
	Expect(err).ToNot(HaveOccurred(), out)
}

// returns all the objects currently stored on the S3 storage
func listS3Blobs() []string {
	createS3HelperPod()

	// list all objects in the bucket (the "mc --quiet ls" command)
	out, err := helpers.Kubectl("exec", minioHelperPod, "--", "mc", "--quiet", "ls", "minio/epinio")
	Expect(err).ToNot(HaveOccurred(), out)

	return strings.Split(string(out), "\n")
}

func appFromAPI(namespace, app string) models.App {
	response, err := env.Curl("GET",
		fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
			serverURL, v1.Root, namespace, app),
		strings.NewReader(""))

	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, response).ToNot(BeNil())
	defer response.Body.Close()
	ExpectWithOffset(1, response.StatusCode).To(Equal(http.StatusOK))
	bodyBytes, err := ioutil.ReadAll(response.Body)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	var responseApp models.App
	err = json.Unmarshal(bodyBytes, &responseApp)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), string(bodyBytes))
	ExpectWithOffset(1, responseApp.Meta.Name).To(Equal(app))
	ExpectWithOffset(1, responseApp.Meta.Namespace).To(Equal(namespace))

	return responseApp
}

func updateAppInstances(namespace string, app string, instances int32) (int, []byte) {
	desired := instances
	data, err := json.Marshal(models.ApplicationUpdateRequest{
		Instances: &desired,
	})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	response, err := env.Curl("PATCH",
		fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
			serverURL, v1.Root, namespace, app),
		strings.NewReader(string(data)))
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, response).ToNot(BeNil())

	defer response.Body.Close()
	bodyBytes, err := ioutil.ReadAll(response.Body)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	return response.StatusCode, bodyBytes
}

func updateAppInstancesNAN(namespace string, app string) (int, []byte) {
	desired := int32(314)
	data, err := json.Marshal(models.ApplicationUpdateRequest{
		Instances: &desired,
	})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	// Hack to make the Instances value non-number
	data = []byte(strings.Replace(string(data), "314", `"thisisnotanumber"`, 1))

	response, err := env.Curl("PATCH",
		fmt.Sprintf("%s%s/namespaces/%s/applications/%s",
			serverURL, v1.Root, namespace, app),
		strings.NewReader(string(data)))
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, response).ToNot(BeNil())

	defer response.Body.Close()
	bodyBytes, err := ioutil.ReadAll(response.Body)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	return response.StatusCode, bodyBytes
}

func createApplication(name string, namespace string, routes []string) (*http.Response, error) {
	request := models.ApplicationCreateRequest{
		Name: name,
		Configuration: models.ApplicationUpdateRequest{
			Routes: routes,
		},
	}
	b, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	body := string(b)

	url := serverURL + v1.Root + "/" + v1.Routes.Path("AppCreate", namespace)
	return env.Curl("POST", url, strings.NewReader(body))
}
