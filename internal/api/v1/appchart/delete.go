package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint DELETE /appcharts/:name
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)
	chartName := c.Param("name")

	log.Infow("delete appchart", "name", chartName)
	defer log.Infow("return")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	log.Infow("check existence", "name", chartName)
	exists, existsError := appchart.Exists(ctx, cluster, chartName)
	if existsError != nil {
		return apierror.InternalError(existsError)
	}
	if !exists {
		return apierror.AppChartIsNotKnown(chartName)
	}

	log.Infow("delete appchart resource", "name", chartName)
	deleteError := appchart.Delete(ctx, cluster, chartName)

	if deleteError != nil {
		return apierror.InternalError(deleteError)
	}

	log.Infow("appchart deleted", "name", chartName)
	response.OK(c)
	return nil
}
