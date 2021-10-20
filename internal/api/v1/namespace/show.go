package namespace

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Show handles the API endpoint GET /namespaces/:org
// It returns the details of the specified namespace
func (hc Controller) Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	org := c.Param("org")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.OrgIsNotKnown(org)
	}

	appNames, apierr := namespaceApps(ctx, cluster, org)
	if err != nil {
		return apierr
	}

	serviceNames, apierr := namespaceServices(ctx, cluster, org)
	if err != nil {
		return apierr
	}

	response.OKReturn(c, models.Namespace{
		Name:     org,
		Apps:     appNames,
		Services: serviceNames,
	})
	return nil
}
