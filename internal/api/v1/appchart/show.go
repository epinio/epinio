package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Show handles the API endpoint GET /appcharts/:name
// It returns the details of the specified appchart.
func (hc Controller) Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	chartName := c.Param("name")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	app, err := appchart.Lookup(ctx, cluster, chartName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app == nil {
		return apierror.AppChartIsNotKnown(chartName)
	}

	response.OKReturn(c, app)
	return nil
}
