package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/gin-gonic/gin"
)

// Index handles the API endpoint GET /appcharts
// It lists all the known appcharts in all namespaces
func (hc Controller) Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	allApps, err := appchart.List(ctx, cluster)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	response.OKReturn(c, allApps)
	return nil
}
