package appchart_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/appchart"
	"github.com/epinio/epinio/internal/helm/helmtest"
	"github.com/epinio/epinio/internal/registry"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// Keep the specs from reading and writing the registry logins of the user running them.
var _ = BeforeSuite(func() {
	restore, err := helmtest.IsolateHelmConfig()
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(restore)
})

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
	Describe("with a chart registry", func() {
		const epinioNamespace = "epinio"

		var (
			reg *helmtest.Registry

			apiMu    sync.Mutex
			apiCalls []string // `<method> <path>`
			created  []string // bodies of the created application charts
		)

		// calls returns the recorded requests for application charts, other than the existence check.
		calls := func() []string {
			apiMu.Lock()
			defer apiMu.Unlock()

			result := []string{}
			for _, call := range apiCalls {
				if !strings.HasPrefix(call, "GET ") {
					result = append(result, call)
				}
			}
			return result
		}

		BeforeEach(func() {
			apiCalls, created = nil, nil

			oldNamespace := viper.GetString("namespace")
			viper.Set("namespace", epinioNamespace)
			DeferCleanup(func() { viper.Set("namespace", oldNamespace) })

			reg = helmtest.NewRegistry("admin", "changeme")
			DeferCleanup(reg.Close)

			// Fake kubernetes API server. No application chart exists. Creation and deletion
			// are recorded.
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiMu.Lock()
				defer apiMu.Unlock()

				apiCalls = append(apiCalls, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")

				switch r.Method {
				case http.MethodPost:
					body, _ := io.ReadAll(r.Body)
					created = append(created, string(body))
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(body)
				case http.MethodDelete:
					fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Success"}`)
				default:
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Failure",`+
						`"reason":"NotFound","code":404}`)
				}
			}))
			DeferCleanup(apiServer.Close)

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: registry.CredentialsSecretName, Namespace: epinioNamespace},
				Data: map[string][]byte{".dockerconfigjson": []byte(
					`{"auths":{"` + reg.Host() + `":{"username":"admin","password":"changeme"}}}`)},
				Type: corev1.SecretTypeDockerConfigJson,
			}

			kubernetes.SetClusterMemo(&kubernetes.Cluster{
				Kubectl:    fake.NewSimpleClientset(secret),
				RestConfig: &rest.Config{Host: apiServer.URL},
			})
			DeferCleanup(func() { kubernetes.SetClusterMemo(nil) })
		})

		It("pushes the chart to a repository of its own, and creates the application chart for it", func() {
			err := push(saveChart(""), map[string]string{"name": "myapp"})
			Expect(err).To(BeNil())
			Expect(recorder.Code).To(Equal(http.StatusOK))

			Expect(reg.HasChart("epinio-charts/myapp/mychart", "0.1.0")).To(BeTrue())

			Expect(created).To(HaveLen(1))
			Expect(created[0]).To(ContainSubstring(`"helmChart":"mychart:0.1.0"`))
			Expect(created[0]).To(ContainSubstring(`"helmRepo":"oci://` + reg.Host() + `/epinio-charts/myapp"`))
			Expect(calls()).To(HaveLen(1))
		})

		It("refuses a chart version which is already stored, and creates nothing", func() {
			Expect(push(saveChart(""), map[string]string{"name": "myapp"})).To(BeNil())
			Expect(created).To(HaveLen(1))

			// The fake cluster does not keep the application chart. The registry is what
			// rejects the second push, for the same chart name and version.
			recorder = httptest.NewRecorder()
			err := push(saveChart(""), map[string]string{"name": "myapp"})
			Expect(err).ToNot(BeNil())
			Expect(err.FirstStatus()).To(Equal(http.StatusConflict))
			Expect(err.Errors()[0].Details).To(ContainSubstring("bump the version"))

			Expect(created).To(HaveLen(1))
			Expect(calls()).To(HaveLen(1))
		})

		It("removes the application chart again when the push fails", func() {
			reg.FailPushes(true)

			err := push(saveChart(""), map[string]string{"name": "myapp"})
			Expect(err).ToNot(BeNil())
			Expect(err.FirstStatus()).To(Equal(http.StatusInternalServerError))

			Expect(reg.HasChart("epinio-charts/myapp/mychart", "0.1.0")).To(BeFalse())
			Expect(calls()).To(HaveLen(2))
			Expect(calls()[0]).To(HavePrefix("POST "))
			Expect(calls()[1]).To(HavePrefix("DELETE "))
			Expect(calls()[1]).To(HaveSuffix("/appcharts/myapp"))
		})

		It("creates nothing when the registry cannot be used", func() {
			reg.SetPassword("something-else")

			err := push(saveChart(""), map[string]string{"name": "myapp"})
			Expect(err).ToNot(BeNil())
			Expect(err.FirstStatus()).To(Equal(http.StatusInternalServerError))
			Expect(calls()).To(BeEmpty())
		})
	})
})
