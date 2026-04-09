package namespace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/namespace"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Create Namespace API", func() {
	var ctx context.Context
	var cluster *kubernetes.Cluster
	var fakeClient *fake.Clientset
	var recorder *httptest.ResponseRecorder
	var c *gin.Context
	var installationNamespace string

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fake.NewSimpleClientset()
		cluster = &kubernetes.Cluster{
			Kubectl: fakeClient,
		}
		// Inject the fake cluster
		kubernetes.SetClusterMemo(cluster)
		
		helpers.Logger = zap.NewNop().Sugar()
		viper.Set("timeout-multiplier", 0)
		
		installationNamespace = "custom-epinio"
		viper.Set("namespace", installationNamespace)

		// Create registry-creds in installation namespace
		_, err := fakeClient.CoreV1().Secrets(installationNamespace).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      registry.CredentialsSecretName,
				Namespace: installationNamespace,
			},
			Data: map[string][]byte{
				".dockerconfigjson": []byte("some-config"),
			},
			Type: corev1.SecretTypeDockerConfigJson,
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		recorder = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(recorder)
		
		// Setup auth service and create user
		authService := auth.NewAuthService(cluster)
		user := auth.User{
			Username: "admin",
			Roles: []auth.Role{
				auth.AdminRole,
			},
		}
		// We need to save the user so the secret exists and secretName is set
		savedUser, err := authService.SaveUser(ctx, user)
		Expect(err).ToNot(HaveOccurred())

		reqCtx := requestctx.WithUser(ctx, savedUser)
		c.Request, _ = http.NewRequestWithContext(reqCtx, "POST", "/namespaces", nil)
	})

	It("creates a namespace successfully", func() {
		namespaceName := "new-namespace"
		requestBody := models.NamespaceCreateRequest{
			Name: namespaceName,
		}
		jsonBody, _ := json.Marshal(requestBody)
		// We need to recreate the request with the body because c.Request was created in BeforeEach
		// But we need to keep the context with the user
		c.Request, _ = http.NewRequestWithContext(c.Request.Context(), "POST", "/namespaces", bytes.NewBuffer(jsonBody))

		err := namespace.Create(c)
		Expect(err).To(BeNil())
		Expect(recorder.Code).To(Equal(http.StatusCreated))

		// Verify namespace created
		_, getErr := fakeClient.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
		Expect(getErr).ToNot(HaveOccurred())
	})

	It("fails with bad request if name is missing", func() {
		requestBody := models.NamespaceCreateRequest{
			Name: "",
		}
		jsonBody, _ := json.Marshal(requestBody)
		c.Request, _ = http.NewRequestWithContext(c.Request.Context(), "POST", "/namespaces", bytes.NewBuffer(jsonBody))

		err := namespace.Create(c)
		Expect(err).ToNot(BeNil())
		Expect(err.Errors()[0].Title).To(ContainSubstring("name of namespace to create not found"))
	})
})
