package builderimage

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/builderimage"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	models "github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Create handles POST /builderimages
func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	client, clientError := cluster.ClientBuilderImage()
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	var createRequest models.BuilderImageCreateRequest
	bindError := c.BindJSON(&createRequest)
	if bindError != nil {
		return apierror.NewBadRequestError(bindError.Error())
	}

	if createRequest.Name == "" {
		return apierror.NewBadRequestError("builder image name is required")
	}
	if createRequest.Image == "" {
		return apierror.NewBadRequestError("image is required")
	}

	exists, existsError := builderimage.Exists(ctx, client, createRequest.Name)
	if existsError != nil {
		return apierror.InternalError(existsError)
	}
	if exists {
		return apierror.BuilderImageAlreadyKnown(createRequest.Name)
	}

	_, createError := builderimage.Create(ctx, client, createRequest)
	if createError != nil {
		return apierror.InternalError(createError)
	}

	response.Created(c)
	return nil
}
