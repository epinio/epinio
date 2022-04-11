package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Create handles the API endpoint POST /appcharts
// It creates a new appchart.
func (hc Controller) Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	var createRequest models.ChartCreateRequest
	err = c.BindJSON(&createRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	found, err := appchart.Exists(ctx, cluster, createRequest.Name)
	if err != nil {
		return apierror.InternalError(err, "failed to check for app resource")
	}
	if found {
		return apierror.AppChartAlreadyKnown(createRequest.Name)
	}

	// Arguments found OK, now we can modify the system state

	err = appchart.Create(ctx, cluster, createRequest.Name,
		createRequest.Repository,
		createRequest.URL,
	)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}
