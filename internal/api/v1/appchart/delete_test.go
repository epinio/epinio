package appchart_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/appchart"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helm/helmtest"
	"github.com/epinio/epinio/internal/registry"
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

var _ = Describe("Delete AppChart API", func() {
	const epinioNamespace = "epinio"

	var (
		reg *helmtest.Registry

		apiMu    sync.Mutex
		apiCalls []string          // `<method> <path>`
		stored   map[string]string // application chart name -> `<helmChart>|<helmRepo>`
	)

	// deletes returns the recorded deletions of application charts.
	deletes := func() []string {
		apiMu.Lock()
		defer apiMu.Unlock()

		result := []string{}
		for _, call := range apiCalls {
			if strings.HasPrefix(call, "DELETE ") {
				result = append(result, call)
			}
		}
		return result
	}

	// seedChart pushes a chart into the repository of the application chart.
	seedChart := func(appChartName string) {
		cluster, err := kubernetes.GetCluster(context.Background())
		Expect(err).ToNot(HaveOccurred())

		r, err := helm.OpenChartRegistry(context.Background(), cluster, appChartName)
		Expect(err).ToNot(HaveOccurred())

		path, err := chartutil.Save(&chart.Chart{
			Metadata: &chart.Metadata{APIVersion: "v2", Name: "mychart", Version: "0.1.0"},
		}, GinkgoT().TempDir())
		Expect(err).ToNot(HaveOccurred())
		Expect(r.Push(path)).To(Succeed())
	}

	// pushedRepo is where a chart for the application chart is stored by Epinio.
	pushedRepo := func(appChartName string) string {
		return "oci://" + reg.Host() + "/epinio-charts/" + appChartName
	}

	deleteAppChart := func(name string) (int, string) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		request, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, "/appcharts/"+name, nil)
		Expect(err).ToNot(HaveOccurred())
		c.Request = request
		c.Params = gin.Params{{Key: "name", Value: name}}

		if apiErr := appchart.Delete(c); apiErr != nil {
			return apiErr.FirstStatus(), apiErr.Errors()[0].Title
		}
		return recorder.Code, ""
	}

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		apiCalls = nil
		stored = map[string]string{}

		oldLogger := helpers.Logger
		helpers.Logger = zap.NewNop().Sugar()
		DeferCleanup(func() { helpers.Logger = oldLogger })

		oldNamespace := viper.GetString("namespace")
		viper.Set("namespace", epinioNamespace)
		DeferCleanup(func() { viper.Set("namespace", oldNamespace) })

		reg = helmtest.NewRegistry("admin", "changeme")
		DeferCleanup(reg.Close)

		// Fake kubernetes API server, holding the application charts of `stored`. Deletions
		// are recorded.
		apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiMu.Lock()
			defer apiMu.Unlock()

			apiCalls = append(apiCalls, r.Method+" "+r.URL.Path)
			w.Header().Set("Content-Type", "application/json")

			name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			spec, found := stored[name]

			switch {
			case r.Method == http.MethodDelete:
				fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Success"}`)
			case found:
				parts := strings.SplitN(spec, "|", 2)
				fmt.Fprintf(w, `{"apiVersion":"application.epinio.io/v1","kind":"AppChart",`+
					`"metadata":{"name":%q,"namespace":"epinio"},`+
					`"spec":{"helmChart":%q,"helmRepo":%q}}`, name, parts[0], parts[1])
			default:
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`)
			}
		}))
		DeferCleanup(apiServer.Close)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: registry.CredentialsSecretName, Namespace: epinioNamespace},
			Data: map[string][]byte{".dockerconfigjson": []byte(
				`{"auths":{"` + reg.URL() + `":{"username":"admin","password":"changeme"}}}`)},
			Type: corev1.SecretTypeDockerConfigJson,
		}

		kubernetes.SetClusterMemo(&kubernetes.Cluster{
			Kubectl:    fake.NewSimpleClientset(secret),
			RestConfig: &rest.Config{Host: apiServer.URL},
		})
		DeferCleanup(func() { kubernetes.SetClusterMemo(nil) })
	})

	It("removes the pushed chart from the registry, and deletes the application chart", func() {
		seedChart("myapp")
		stored["myapp"] = "mychart:0.1.0|" + pushedRepo("myapp")
		Expect(reg.HasChart("epinio-charts/myapp/mychart", "0.1.0")).To(BeTrue())

		status, _ := deleteAppChart("myapp")

		Expect(status).To(Equal(http.StatusOK))
		Expect(reg.Tags("epinio-charts/myapp/mychart")).To(BeEmpty())
		Expect(deletes()).To(HaveLen(1))
		Expect(deletes()[0]).To(HaveSuffix("/appcharts/myapp"))
	})

	It("leaves the charts of other application charts in the registry", func() {
		seedChart("myapp")
		seedChart("other")
		stored["myapp"] = "mychart:0.1.0|" + pushedRepo("myapp")

		status, _ := deleteAppChart("myapp")

		Expect(status).To(Equal(http.StatusOK))
		Expect(reg.HasChart("epinio-charts/myapp/mychart", "0.1.0")).To(BeFalse())
		Expect(reg.HasChart("epinio-charts/other/mychart", "0.1.0")).To(BeTrue())
	})

	It("keeps the application chart when the chart cannot be removed from the registry", func() {
		seedChart("myapp")
		stored["myapp"] = "mychart:0.1.0|" + pushedRepo("myapp")
		reg.SetPassword("something-else")

		status, _ := deleteAppChart("myapp")

		Expect(status).To(Equal(http.StatusInternalServerError))
		Expect(deletes()).To(BeEmpty())
		Expect(reg.HasChart("epinio-charts/myapp/mychart", "0.1.0")).To(BeTrue())
	})

	It("does not touch the registry for a chart which was not pushed", func() {
		seedChart("myapp")
		stored["myapp"] = "mychart:0.1.0|oci://ghcr.io/epinio/charts"
		reg.SetPassword("something-else") // Any use of the registry would fail the request.

		status, _ := deleteAppChart("myapp")

		Expect(status).To(Equal(http.StatusOK))
		Expect(deletes()).To(HaveLen(1))
		Expect(reg.HasChart("epinio-charts/myapp/mychart", "0.1.0")).To(BeTrue())
	})

	It("does not touch the registry for a chart given by url", func() {
		stored["myapp"] = "https://example.com/chart.tgz|"
		reg.SetPassword("something-else")

		status, _ := deleteAppChart("myapp")

		Expect(status).To(Equal(http.StatusOK))
		Expect(deletes()).To(HaveLen(1))
	})

	It("reports an application chart which does not exist", func() {
		status, _ := deleteAppChart("unknown")

		Expect(status).To(Equal(http.StatusNotFound))
		Expect(deletes()).To(BeEmpty())
	})
})
