package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/gin-gonic/gin"
)

// FullIndex handles the API endpoint GET /applications
// It lists all the known applications in all namespaces, with and without workload.
func (hc Controller) FullIndex(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	allApps, err := application.List(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, allApps)
	return nil
}
