package namespace

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	epinioerrors "github.com/epinio/epinio/internal/errors"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Index handles the API endpoint /namespaces (GET)
// It returns a list of all Epinio-controlled namespaces
// An Epinio namespace is nothing but a kubernetes namespace which has a
// special Label (Look at the code to see which).
func (oc Controller) Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	user := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	namespaceList, err := namespaces.List(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err)
	}
	namespaceList = auth.FilterResources(user, namespaceList)

	namespaces := make(models.NamespaceList, 0, len(namespaceList))
	for _, namespace := range namespaceList {
		appNames, err := namespaceApps(ctx, cluster, namespace.Name)
		if err != nil {
			return apierror.InternalError(err)
		}

		configurationNames, err := namespaceConfigurations(ctx, cluster, namespace.Name)
		// Ignore namespace if deleted mid-flight
		if _, ok := err.(epinioerrors.NamespaceMissingError); ok {
			continue
		}
		if err != nil {
			return apierror.InternalError(err)
		}

		namespaces = append(namespaces, models.Namespace{
			Meta: models.MetaLite{
				Name:      namespace.Name,
				CreatedAt: namespace.CreatedAt,
			},
			Apps:           appNames,
			Configurations: configurationNames,
		})
	}

	response.OKReturn(c, namespaces)
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
