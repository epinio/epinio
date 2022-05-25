package namespace

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
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
		return apierror.InternalError(err)
	}

	appNames, err := namespaceApps(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	configurationNames, err := namespaceConfigurations(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	space, err := namespaces.Get(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
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
