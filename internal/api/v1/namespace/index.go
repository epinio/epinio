package namespace

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	epinioerrors "github.com/epinio/epinio/internal/errors"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
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
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	orgList, err := organizations.List(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	namespaces := make(models.NamespaceList, 0, len(orgList))
	for _, org := range orgList {
		appNames, err := namespaceApps(ctx, cluster, org.Name)
		if err != nil {
			return apierror.InternalError(err)
		}

		serviceNames, err := namespaceServices(ctx, cluster, org.Name)
		// Ignore namespace if deleted mid-flight
		if _, ok := err.(epinioerrors.NamespaceMissingError); ok {
			continue
		}
		if err != nil {
			return apierror.InternalError(err)
		}

		namespaces = append(namespaces, models.Namespace{
			Name:     org.Name,
			Apps:     appNames,
			Services: serviceNames,
		})
	}

	response.OKReturn(c, namespaces)
	return nil
}

func namespaceApps(ctx context.Context, cluster *kubernetes.Cluster, org string) ([]string, error) {
	// Retrieve app references for namespace, and reduce to their names.
	appRefs, err := application.ListAppRefs(ctx, cluster, org)
	if err != nil {
		return nil, err
	}
	appNames := make([]string, 0, len(appRefs))
	for _, app := range appRefs {
		appNames = append(appNames, app.Name)
	}

	return appNames, nil
}

func namespaceServices(ctx context.Context, cluster *kubernetes.Cluster, org string) ([]string, error) {
	// Retrieve services for namespace, and reduce to their names.
	services, err := services.List(ctx, cluster, org)
	if err != nil {
		return nil, err
	}
	serviceNames := make([]string, 0, len(services))
	for _, service := range services {
		serviceNames = append(serviceNames, service.Name())
	}

	return serviceNames, nil
}
