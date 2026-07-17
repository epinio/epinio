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

func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	log.Infow("create appchart")
	defer log.Infow("return")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	client, clientError := cluster.ClientAppChart()
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	var createRequest models.AppChartCreateRequest
	bindError := c.BindJSON(&createRequest)
	if bindError != nil {
		return apierror.NewBadRequestError(bindError.Error())
	}

	log.Infow("check existence", "name", createRequest.Name)
	exists, existsError := appchart.Exists(ctx, client, createRequest.Name)
	if existsError != nil {
		return apierror.InternalError(existsError)
	}
	if exists {
		return apierror.AppChartAlreadyKnown(createRequest.Name)
	}

	log.Infow("create appchart resource", "name", createRequest.Name)
	_, appChartError := appchart.Create(ctx, client, createRequest)

	if appChartError != nil {
		return apierror.InternalError(appChartError)
	}

	log.Infow("appchart created", "name", createRequest.Name)
	response.Created(c)
	return nil
}
