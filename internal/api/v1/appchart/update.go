package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	models "github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
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

	client, clientError := cluster.ClientAppChart()
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	log.Infow("check existence", "name", chartName)
	exists, existsError := appchart.Exists(ctx, client, chartName)
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
	updateError := appchart.UpdateWithChart(
		ctx,
		client,
		chartStoreOpener(cluster),
		boundAppsOf(cluster),
		chartName,
		updateRequest,
	)

	if updateError != nil {
		var inUse *appchart.InUseError
		if errors.As(updateError, &inUse) {
			return inUseError(inUse)
		}

		var locked *appchart.LocationLockedError
		if errors.As(updateError, &locked) {
			return apierror.NewBadRequestErrorf(
				"the chart of application chart '%s' is stored by Epinio, its chart and repository cannot be changed",
				locked.Name,
			).WithDetails("push the new chart to the same name to replace it")
		}
		return apierror.InternalError(updateError)
	}

	log.Infow("appchart updated", "name", chartName)
	response.OK(c)
	return nil
}
