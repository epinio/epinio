package v1

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/namespaces"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// NamespaceMiddleware is a gin middleware used to check if a namespaced route is valid.
// It checks the validity of the requested namespace, returning a 404 if it doesn't exists
func NamespaceMiddleware(c *gin.Context) {
	_ = requestctx.Logger(c.Request.Context()).WithName("NamespaceMiddleware")
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	if namespace == "" {
		return
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		response.Error(c, apierrors.InternalError(err))
		c.Abort()
		return
	}

	exists, err := namespaces.Exists(ctx, cluster, namespace)
	if err != nil {
		response.Error(c, apierrors.InternalError(err))
		c.Abort()
		return
	}

	if !exists {
		response.Error(c, apierrors.NamespaceIsNotKnown(namespace))
		c.Abort()
	}
}
