package namespace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/namespace"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

var _ = Describe("Index Namespace API", func() {
	var recorder *httptest.ResponseRecorder
	var c *gin.Context

	epinioNamespace := func(name string) *corev1.Namespace {
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					kubernetes.EpinioNamespaceLabelKey: kubernetes.
						EpinioNamespaceLabelValue,
				},
			},
		}
	}

	// Service instances are labeled secrets; the secret name is the
	// generated resource name, the label carries the user-facing one.
	serviceSecret := func(name, ns string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "s-" + name + "-generated",
				Namespace: ns,
				Labels: map[string]string{
					services.ServiceNameLabelKey:    name,
					services.CatalogServiceLabelKey: "mysql-dev",
				},
			},
		}
	}

	configSecret := func(name, ns string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				Labels: map[string]string{
					configurations.ConfigurationLabelKey: "true",
				},
			},
		}
	}

	// Apps are a CRD read through the dynamic client, which the typed fake
	// does not cover. A fake API server answers the app list instead.
	appsServer := func(apps map[string]string) *httptest.Server {
		items := ""
		for name, ns := range apps {
			if items != "" {
				items += ","
			}
			items += fmt.Sprintf(
				`{"apiVersion":"application.epinio.io/v1","kind":"App",`+
					`"metadata":{"name":%q,"namespace":%q}}`,
				name,
				ns,
			)
		}
		body := `{"apiVersion":"application.epinio.io/v1",` +
			`"kind":"AppList","metadata":{},"items":[` + items + `]}`

		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, body)
			},
		))
		DeferCleanup(server.Close)
		return server
	}

	setup := func(apps map[string]string, objs ...runtime.Object) {
		kubernetes.SetClusterMemo(&kubernetes.Cluster{
			Kubectl:    fake.NewSimpleClientset(objs...),
			RestConfig: &rest.Config{Host: appsServer(apps).URL},
		})
		DeferCleanup(func() { kubernetes.SetClusterMemo(nil) })
	}

	request := func(query string) {
		user := auth.User{
			Username: "admin",
			Roles:    []auth.Role{auth.AdminRole},
		}
		ctx := requestctx.WithUser(context.Background(), user)

		recorder = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(recorder)
		c.Request, _ = http.NewRequestWithContext(
			ctx,
			"GET",
			"/namespaces"+query,
			nil,
		)

		Expect(namespace.Index(c)).To(BeNil())
		Expect(recorder.Code).To(Equal(http.StatusOK))
	}

	byName := func(list models.NamespaceList) map[string]models.Namespace {
		out := map[string]models.Namespace{}
		for _, ns := range list {
			ns.Meta.CreatedAt = metav1.Time{}
			out[ns.Meta.Name] = ns
		}
		return out
	}

	expected := map[string]models.Namespace{
		"workspace": {
			Meta:           models.MetaLite{Name: "workspace"},
			Apps:           []string{"web"},
			Services:       []string{"mysql"},
			Configurations: []string{"creds"},
		},
		"other": {
			Meta:     models.MetaLite{Name: "other"},
			Services: []string{"redis"},
		},
		"empty": {
			Meta: models.MetaLite{Name: "empty"},
		},
	}

	BeforeEach(func() {
		setup(
			map[string]string{"web": "workspace", "stray": "not-epinio"},
			epinioNamespace("workspace"),
			epinioNamespace("other"),
			epinioNamespace("empty"),
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "not-epinio"},
			},
			serviceSecret("mysql", "workspace"),
			serviceSecret("redis", "other"),
			serviceSecret("stray", "not-epinio"),
			configSecret("creds", "workspace"),
		)
	})

	It("lists apps, services and configurations per namespace", func() {
		request("")

		var list models.NamespaceList
		Expect(json.Unmarshal(recorder.Body.Bytes(), &list)).To(Succeed())
		Expect(byName(list)).To(Equal(expected))
	})

	It("returns the same per-namespace names on a page", func() {
		request("?page=1&pageSize=2")

		var resp response.PaginatedResponse[models.Namespace]
		Expect(json.Unmarshal(recorder.Body.Bytes(), &resp)).To(Succeed())
		Expect(resp.TotalItems).To(Equal(3))
		Expect(resp.TotalPages).To(Equal(2))
		Expect(resp.Items).To(HaveLen(2))

		for name, ns := range byName(resp.Items) {
			Expect(ns).To(Equal(expected[name]))
		}
	})

	It("returns an empty page past the end", func() {
		request("?page=3&pageSize=2")

		var resp response.PaginatedResponse[models.Namespace]
		Expect(json.Unmarshal(recorder.Body.Bytes(), &resp)).To(Succeed())
		Expect(resp.TotalItems).To(Equal(3))
		Expect(resp.Items).To(BeEmpty())
	})

	Describe("filtering", func() {
		names := func(list models.NamespaceList) []string {
			out := make([]string, 0, len(list))
			for _, ns := range list {
				out = append(out, ns.Meta.Name)
			}
			return out
		}

		plain := func(query string) []string {
			request(query)

			var list models.NamespaceList
			Expect(json.Unmarshal(recorder.Body.Bytes(), &list)).
				To(Succeed())
			return names(list)
		}

		It("matches search as a case-insensitive substring", func() {
			Expect(plain("?search=SPACE")).To(ConsistOf("workspace"))
			Expect(plain("?search=o")).To(ConsistOf("workspace", "other"))
		})

		It("returns nothing when search matches no namespace", func() {
			Expect(plain("?search=nope")).To(BeEmpty())
		})

		It("never matches a namespace Epinio does not own", func() {
			Expect(plain("?search=not-epinio")).To(BeEmpty())
		})

		It("keeps the per-namespace names on a search hit", func() {
			request("?search=work")

			var list models.NamespaceList
			Expect(json.Unmarshal(recorder.Body.Bytes(), &list)).
				To(Succeed())
			Expect(byName(list)).To(Equal(map[string]models.Namespace{
				"workspace": expected["workspace"],
			}))
		})

		It("paginates the search result, not the full list", func() {
			request("?search=o&page=1&pageSize=1")

			var resp response.PaginatedResponse[models.Namespace]
			Expect(json.Unmarshal(recorder.Body.Bytes(), &resp)).
				To(Succeed())
			Expect(resp.TotalItems).To(Equal(2))
			Expect(resp.TotalPages).To(Equal(2))
			Expect(names(resp.Items)).To(HaveLen(1))
			Expect(names(resp.Items)[0]).To(BeElementOf("workspace", "other"))
		})

		It("matches namespaces= by exact name only", func() {
			Expect(plain("?namespaces=other")).To(ConsistOf("other"))
			Expect(plain("?namespaces=other,empty")).
				To(ConsistOf("other", "empty"))
			Expect(plain("?namespaces=work")).To(BeEmpty())
		})

		It("applies namespaces= and search together", func() {
			Expect(plain("?namespaces=workspace,other&search=oth")).
				To(ConsistOf("other"))
		})
	})
})
