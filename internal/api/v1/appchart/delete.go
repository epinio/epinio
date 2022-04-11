package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint DELETE /appcharts/:name
// It removes the named appchart
func (hc Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	chartName := c.Param("name")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	found, err := appchart.Exists(ctx, cluster, chartName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if !found {
		return apierror.AppChartIsNotKnown(chartName)
	}

	err = appchart.Delete(ctx, cluster, chartName)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}
