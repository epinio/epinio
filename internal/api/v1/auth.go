package v1

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"
)

func AuthorizationMiddleware(enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := requestctx.Logger(c.Request.Context())
		user := requestctx.User(c.Request.Context())

		subject := user.Role
		domains := user.Namespaces
		action := c.Request.Method
		resource := c.Request.URL.Path

		var authorized bool
		var err error

		logger.Info(fmt.Sprintf("authorization request for subject:[%s] domain:[%#v-len(%d)] action:[%s] resource:[%s]\n", subject, domains, len(domains), action, resource))

		for _, domain := range domains {
			authorized, err = enforcer.Enforce(subject, domain, action, resource)
			logger.Info(fmt.Sprintf("authorized:[%+v] - err:[%+v]\n", authorized, err))
		}
	}
}
