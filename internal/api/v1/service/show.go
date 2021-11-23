package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Show handles the API end point /namespaces/:namespace/services/:service
// It returns the detail information of the named service instance
func (sc Controller) Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	serviceName := c.Param("service")

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
	if err != nil {
		if err.Error() == "service not found" {
			return apierror.ServiceIsNotKnown(serviceName)
		}
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	appsOf, err := servicesToApps(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	serviceDetails, err := service.Details(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	var appNames []string
	for _, app := range appsOf[serviceName] {
		appNames = append(appNames, app.Meta.Name)
	}

	response.OKReturn(c, models.ServiceShowResponse{
		Username:  service.User(),
		Details:   serviceDetails,
		BoundApps: appNames,
	})
	return nil
}
