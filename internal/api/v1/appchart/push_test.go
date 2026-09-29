package appchart_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

var _ = Describe("Push AppChart API", func() {
	var (
		tmpDir   string
		recorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)

		oldLogger := helpers.Logger
		helpers.Logger = zap.NewNop().Sugar()
		DeferCleanup(func() { helpers.Logger = oldLogger })

		tmpDir = GinkgoT().TempDir()
		recorder = httptest.NewRecorder()
	})

	// saveChart writes a chart archive with the given type into the temp directory,
	// and returns its content
	saveChart := func(chartType string) []byte {
		path, err := chartutil.Save(&chart.Chart{
			Metadata: &chart.Metadata{
				APIVersion: "v2",
				Name:       "mychart",
				Version:    "0.1.0",
				Type:       chartType,
			},
		}, tmpDir)
		Expect(err).ToNot(HaveOccurred())

		content, err := os.ReadFile(filepath.Clean(path))
		Expect(err).ToNot(HaveOccurred())
		return content
	}

	// push invokes the handler with a multipart request. A nil file leaves out the file part.
	push := func(file []byte, fields map[string]string) apierror.APIErrors {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		if file != nil {
			part, err := writer.CreateFormFile("file", "mychart-0.1.0.tgz")
			Expect(err).ToNot(HaveOccurred())
			_, err = part.Write(file)
			Expect(err).ToNot(HaveOccurred())
		}
		for key, value := range fields {
			Expect(writer.WriteField(key, value)).To(Succeed())
		}
		Expect(writer.Close()).To(Succeed())

		c, _ := gin.CreateTestContext(recorder)
		request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/appcharts/push", body)
		Expect(err).ToNot(HaveOccurred())
		request.Header.Set("Content-Type", writer.FormDataContentType())
		c.Request = request

		return appchart.Push(c)
	}

	expectBadRequest := func(err apierror.APIErrors, message string) {
		ExpectWithOffset(1, err).ToNot(BeNil())
		ExpectWithOffset(1, err.FirstStatus()).To(Equal(http.StatusBadRequest))
		ExpectWithOffset(1, err.Errors()[0].Title+" "+err.Errors()[0].Details).To(ContainSubstring(message))
	}

	It("rejects a request without file", func() {
		err := push(nil, map[string]string{"name": "mychart"})
		expectBadRequest(err, "can't read multipart file input")
	})

	It("rejects a request without name", func() {
		err := push(saveChart(""), map[string]string{})
		expectBadRequest(err, "name is required")
	})

	DescribeTable("rejects an invalid name",
		func(name string) {
			err := push(saveChart(""), map[string]string{"name": name})
			expectBadRequest(err, "invalid application chart name")
		},
		Entry("upper case", "MyChart"),
		Entry("underscore", "my_chart"),
		Entry("leading dash", "-mychart"),
		Entry("trailing dot", "mychart."),
	)

	It("rejects a file which is not a chart archive", func() {
		err := push([]byte("hello"), map[string]string{"name": "mychart"})
		expectBadRequest(err, "the upload is not a valid helm chart archive")
	})

	It("rejects a library chart", func() {
		err := push(saveChart("library"), map[string]string{"name": "mychart"})
		expectBadRequest(err, "is not usable as application chart")
	})

	It("does not touch the cluster for invalid input", func() {
		// A cluster which fails every call, injected to see whether it is used at all.
		kubernetes.SetClusterMemo(&kubernetes.Cluster{
			Kubectl:    fake.NewSimpleClientset(),
			RestConfig: &rest.Config{Host: "http://127.0.0.1:1"},
		})
		DeferCleanup(func() { kubernetes.SetClusterMemo(nil) })

		err := push([]byte("hello"), map[string]string{"name": "mychart"})
		expectBadRequest(err, "the upload is not a valid helm chart archive")
	})

	Describe("with an existing application chart of the same name", func() {
		var apiServer *httptest.Server

		BeforeEach(func() {
			// Fake kubernetes API server answering every GET of an appchart with an existing one.
			apiServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"apiVersion":"application.epinio.io/v1","kind":"AppChart",`+
					`"metadata":{"name":"mychart","namespace":"epinio"},"spec":{}}`)
			}))
			DeferCleanup(apiServer.Close)

			kubernetes.SetClusterMemo(&kubernetes.Cluster{
				Kubectl:    fake.NewSimpleClientset(),
				RestConfig: &rest.Config{Host: apiServer.URL},
			})
			DeferCleanup(func() { kubernetes.SetClusterMemo(nil) })
		})

		It("reports a conflict, before anything is pushed", func() {
			err := push(saveChart(""), map[string]string{"name": "mychart"})
			Expect(err).ToNot(BeNil())
			Expect(err.FirstStatus()).To(Equal(http.StatusConflict))
		})
	})
})
