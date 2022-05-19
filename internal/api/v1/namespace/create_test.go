package namespace_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/epinio/epinio/internal/api/v1/namespace"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/stdr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespace Controller", func() {
	var c *gin.Context
	var ctx context.Context
	var w *httptest.ResponseRecorder
	var url string
	var controller Controller

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		ctx = requestctx.WithLogger(context.Background(), stdr.New(nil))
		url = "http://url.com/endpoint"
	})

	JustBeforeEach(func() {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		Expect(err).ToNot(HaveOccurred())
		c.Request = req.Clone(ctx)
	})

	Context("Create", func() {

		When("url is not restricted", func() {
			It("returns status code 200", func() {
				err := controller.Create(c)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
