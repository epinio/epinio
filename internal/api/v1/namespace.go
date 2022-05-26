package v1

import (
	"fmt"

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
	logger := requestctx.Logger(c.Request.Context()).WithName("NamespaceMiddleware")
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	if namespace == "" {
		return
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		logger.Info("unable to get cluster", "error", err)
		response.Error(c, apierrors.InternalError(err))
		c.Abort()
		return
	}

	exists, err := namespaces.Exists(ctx, cluster, namespace)
	if err != nil {
		logger.Info("unable to check if namespace exists", "error", err)
		response.Error(c, apierrors.InternalError(err))
		c.Abort()
		return
	}

	if !exists {
		logger.Info(fmt.Sprintf("namespace [%s] doesn't exists", namespace))
		response.Error(c, apierrors.NamespaceIsNotKnown(namespace))
		c.Abort()
	}
}
