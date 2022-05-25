package configuration

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Index handles the API end point /namespaces/:namespace/configurations
// It returns a list of all known configuration instances
func (sc Controller) Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	namespaceConfigurations, err := configurations.List(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	appsOf, err := application.BoundAppsNames(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	responseData, err := makeResponse(ctx, appsOf, namespaceConfigurations)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, responseData)
	return nil
}

func makeResponse(ctx context.Context, appsOf map[string][]string, configurations configurations.ConfigurationList) (models.ConfigurationResponseList, error) {

	response := models.ConfigurationResponseList{}

	for _, configuration := range configurations {
		configurationDetails, err := configuration.Details(ctx)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue // Configuration was deleted, ignore it
			} else {
				return models.ConfigurationResponseList{}, err
			}
		}

		key := application.ConfigurationKey(configuration.Name, configuration.Namespace())
		appNames := appsOf[key]

		response = append(response, models.ConfigurationResponse{
			Meta: models.ConfigurationRef{
				Meta: models.Meta{
					CreatedAt: configuration.CreatedAt,
					Name:      configuration.Name,
					Namespace: configuration.Namespace(),
				},
			},
			Configuration: models.ConfigurationShowResponse{
				Username:  configuration.User(),
				Details:   configurationDetails,
				BoundApps: appNames,
			},
		})
	}

	return response, nil
}
