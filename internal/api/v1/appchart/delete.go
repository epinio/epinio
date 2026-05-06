package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint DELETE /appcharts/:name
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	chartName := c.Param("name")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	exists, existsError := appchart.Exists(ctx, cluster, chartName)
	if existsError != nil {
		return apierror.InternalError(existsError)
	}
	if !exists {
		return apierror.AppChartIsNotKnown(chartName)
	}

	deleteError := appchart.Delete(ctx, cluster, chartName)

	if deleteError != nil {
		return apierror.InternalError(deleteError)
	}

	response.OK(c)
	return nil
}
