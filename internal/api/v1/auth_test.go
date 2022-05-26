package v1_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/stdr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Authorization Middleware", func() {
	var c *gin.Context
	var ctx context.Context
	var w *httptest.ResponseRecorder
	var url string

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

	Context("user has Role 'user'", func() {

		BeforeEach(func() {
			ctx = requestctx.WithUser(ctx, auth.User{
				Role:       "user",
				Namespaces: []string{"workspace"},
			})
		})

		When("url is not restricted", func() {
			It("returns status code 200", func() {
				v1.AuthorizationMiddleware(c)
				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})

		When("url is restricted", func() {
			BeforeEach(func() {
				v1.AdminRoutes = map[string]struct{}{
					"/restricted": {},
				}
				url = "http://url.com/restricted"
			})

			It("returns status code 403", func() {
				v1.AuthorizationMiddleware(c)
				Expect(w.Code).To(Equal(http.StatusForbidden))
			})
		})

		When("url is namespaced", func() {
			It("returns status code 403 for another namespace", func() {
				c.Params = []gin.Param{{Key: "namespace", Value: "another-workspace"}}

				v1.AuthorizationMiddleware(c)
				Expect(w.Code).To(Equal(http.StatusForbidden))
			})

			It("returns status code 200 for its namespace", func() {
				c.Params = []gin.Param{{Key: "namespace", Value: "workspace"}}

				v1.AuthorizationMiddleware(c)
				Expect(w.Code).To(Equal(http.StatusOK))
			})
		})
	})
})
