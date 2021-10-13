package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Index handles the API endpoint GET /namespaces/:org/applications
// It lists all the known applications in the specified namespace, with and without workload.
func (hc Controller) Index(c *gin.Context) apierror.APIErrors {
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

	apps, err := application.List(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = response.JSON(c, apps)
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
