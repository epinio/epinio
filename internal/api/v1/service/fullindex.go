package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
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

	appsOf, err := application.BoundAppsNames(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}

	var responseData models.ServiceResponseList

	for _, service := range allServices {

		serviceDetails, err := service.Details(ctx)
		if err != nil {
			return apierror.InternalError(err)
		}

		// NOTE that the `appsOf` map is keyed here by service and namespace, and
		// not the service alone. This is done to distinguish between services of
		// the same name in different namespaces, with different binding states.

		key := application.ServiceKey(service.Name(), service.Namespace())
		appNames := appsOf[key]

		responseData = append(responseData, models.ServiceResponse{
			Meta: models.ServiceRef{
				Name:      service.Name(),
				Namespace: service.Namespace(),
			},
			Spec: models.ServiceShowResponse{
				Username:  service.User(),
				Details:   serviceDetails,
				BoundApps: appNames,
			},
		})
	}

	response.OKReturn(c, responseData)
	return nil
}
