package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	models "github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	var createRequest models.AppChartCreateRequest
	bindError := c.BindJSON(&createRequest)
	if bindError != nil {
		return apierror.NewBadRequestError(bindError.Error())
	}

	exists, existsErr := appchart.Exists(ctx, cluster, createRequest.Name)
	if existsErr != nil {
		return apierror.InternalError(existsErr)
	}
	if exists {
		return apierror.AppChartAlreadyKnown(createRequest.Name)
	}

	_, appChartError := appchart.Create(ctx, cluster, createRequest)

	if appChartError != nil {
		return apierror.InternalError(appChartError)
	}

	response.Created(c)
	return nil
}
