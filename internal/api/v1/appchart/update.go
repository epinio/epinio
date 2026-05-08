package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	models "github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Update handles the API endpoint PATCH /appcharts/:name
func Update(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)
	chartName := c.Param("name")

	log.Infow("update appchart", "name", chartName)
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

	var updateRequest models.AppChartUpdateRequest
	bindError := c.BindJSON(&updateRequest)
	if bindError != nil {
		return apierror.NewBadRequestError(bindError.Error())
	}

	log.Infow("apply update", "name", chartName)
	updateError := appchart.Update(ctx, cluster, chartName, updateRequest)
	if updateError != nil {
		return apierror.InternalError(updateError)
	}

	log.Infow("appchart updated", "name", chartName)
	response.OK(c)
	return nil
}
