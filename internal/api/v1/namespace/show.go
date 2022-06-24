package namespace

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Show handles the API endpoint GET /namespaces/:namespace
// It returns the details of the specified namespace
func (hc Controller) Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	appNames, err := namespaceApps(ctx, cluster, namespace)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	configurationNames, err := namespaceConfigurations(ctx, cluster, namespace)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	space, err := namespaces.Get(ctx, cluster, namespace)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	response.OKReturn(c, models.Namespace{
		Meta: models.MetaLite{
			Name:      namespace,
			CreatedAt: space.CreatedAt,
		},
		Apps:           appNames,
		Configurations: configurationNames,
	})
	return nil
}

func namespaceApps(ctx context.Context, cluster *kubernetes.Cluster, namespace string) ([]string, error) {
	// Retrieve app references for namespace, and reduce to their names.
	appRefs, err := application.ListAppRefs(ctx, cluster, namespace)
	if err != nil {
		return nil, err
	}
	appNames := make([]string, 0, len(appRefs))
	for _, app := range appRefs {
		appNames = append(appNames, app.Name)
	}

	return appNames, nil
}

func namespaceConfigurations(ctx context.Context, cluster *kubernetes.Cluster, namespace string) ([]string, error) {
	// Retrieve configurations for namespace, and reduce to their names.
	configurations, err := configurations.List(ctx, cluster, namespace)
	if err != nil {
		return nil, err
	}
	configurationNames := make([]string, 0, len(configurations))
	for _, configuration := range configurations {
		configurationNames = append(configurationNames, configuration.Name)
	}

	return configurationNames, nil
}
