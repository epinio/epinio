package service

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/api/v1/servicebinding"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Delete handles the API end point /namespaces/:namespace/services/:service (DELETE)
// It deletes the named service
func (sc Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	serviceName := c.Param("service")
	username := requestctx.User(ctx)

	var deleteRequest models.ServiceDeleteRequest
	err := c.BindJSON(&deleteRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

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

	service, err := services.Lookup(ctx, cluster, namespace, serviceName)
	if err != nil && err.Error() == "service not found" {
		return apierror.ServiceIsNotKnown(serviceName)
	}
	if err != nil {
		return apierror.InternalError(err)
	}

	// Verify that the service is unbound. IOW not bound to any application.
	// If it is, and automatic unbind was requested, do that.
	// Without automatic unbind such applications are reported as error.

	boundAppNames, err := application.BoundAppsNamesFor(ctx, cluster, namespace, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if len(boundAppNames) > 0 {
		if !deleteRequest.Unbind {
			return apierror.NewBadRequest("bound applications exist", strings.Join(boundAppNames, ","))
		}

		for _, appName := range boundAppNames {
			apiErr := servicebinding.DeleteBinding(ctx, cluster, namespace, appName, serviceName, username)
			if apiErr != nil {
				return apiErr
			}
		}
	}

	// Everything looks to be ok. Delete.

	err = service.Delete(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, models.ServiceDeleteResponse{
		BoundApps: boundAppNames,
	})
	return nil
}
