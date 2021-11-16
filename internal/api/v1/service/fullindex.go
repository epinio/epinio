package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// FullIndex handles the API endpoint GET /services
// It lists all the known applications in all namespaces, with and without workload.
func (hc Controller) FullIndex(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	allServices, err := services.List(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}

	appsOf, err := servicesToApps(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}

	var responseData models.ServiceResponseList

	for _, service := range allServices {
		var appNames []string

		// NOTE that `appsOf` is keyed here by service and
		// namespace, not service alone. Done to distinguish
		// between services of the same name in different
		// namespaces, with different binding states.

		key := serviceKey(service.Name(), service.Namespace())
		for _, app := range appsOf[key] {
			appNames = append(appNames, app.Meta.Name)
		}
		responseData = append(responseData, models.ServiceResponse{
			Meta: models.ServiceRef{
				Name:      service.Name(),
				Namespace: service.Namespace(),
			},
			BoundApps: appNames,
		})
	}

	response.OKReturn(c, responseData)
	return nil
}
