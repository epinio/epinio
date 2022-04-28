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

		for _, domain := range domains {
			logger.Info(fmt.Sprintf("authorization request for subject:[%s/%s] domain:[%s] action:[%s] resource:[%s]\n", user, subject, domain, action, resource))
			authorized, err := enforcer.Enforce(subject, domain, action, resource)
			logger.Info(fmt.Sprintf("authorized, err: %+v %+v\n", authorized, err))
		}
	}
}
