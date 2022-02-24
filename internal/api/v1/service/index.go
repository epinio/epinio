package service

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Index handles the API end point /namespaces/:namespace/services
// It returns a list of all known service instances
func (sc Controller) Index(c *gin.Context) apierror.APIErrors {
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

	namespaceServices, err := services.List(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	appsOf, err := application.BoundAppsNames(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	responseData, err := makeResponse(ctx, appsOf, namespaceServices)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, responseData)
	return nil
}

func makeResponse(ctx context.Context, appsOf map[string][]string, services services.ServiceList) (models.ServiceResponseList, error) {

	response := models.ServiceResponseList{}

	for _, service := range services {
		serviceDetails, err := service.Details(ctx)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue // Service was deleted, ignore it
			} else {
				return models.ServiceResponseList{}, err
			}
		}

		key := application.ServiceKey(service.Name(), service.Namespace())
		appNames := appsOf[key]

		response = append(response, models.ServiceResponse{
			Meta: models.ServiceRef{
				Name:      service.Name(),
				Namespace: service.Namespace(),
			},
			Configuration: models.ServiceShowResponse{
				Username:  service.User(),
				Details:   serviceDetails,
				BoundApps: appNames,
			},
		})
	}

	return response, nil
}
