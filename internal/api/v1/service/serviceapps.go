package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// ServiceApps handles the API endpoint GET /namespaces/:namespace/serviceapps
// It returns a map from services to the apps they are bound to, in the specified
// namespace.
func (hc Controller) ServiceApps(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := namespaces.Exists(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.NamespaceIsNotKnown(namespace)
	}

	appsOf, err := application.BoundApps(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, appsOf)
	return nil
}
