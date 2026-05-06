package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	models "github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Update handles the API endpoint PATCH /appcharts/:name
func Update(c *gin.Context) apierror.APIErrors {
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

	var updateRequest models.AppChartUpdateRequest
	bindError := c.BindJSON(&updateRequest)
	if bindError != nil {
		return apierror.NewBadRequestError(bindError.Error())
	}

	updateError := appchart.Update(ctx, cluster, chartName, updateRequest)
	if updateError != nil {
		return apierror.InternalError(updateError)
	}

	response.OK(c)
	return nil
}
