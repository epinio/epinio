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

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	var createRequest models.BuilderImageCreateRequest
	if bindErr := c.BindJSON(&createRequest); bindErr != nil {
		return apierror.NewBadRequestError(bindErr.Error())
	}

	if createRequest.Name == "" {
		return apierror.NewBadRequestError("builder image name is required")
	}
	if createRequest.Image == "" {
		return apierror.NewBadRequestError("image is required")
	}

	exists, existsErr := builderimage.Exists(ctx, cluster, createRequest.Name)
	if existsErr != nil {
		return apierror.InternalError(existsErr)
	}
	if exists {
		return apierror.BuilderImageAlreadyKnown(createRequest.Name)
	}

	_, createErr := builderimage.Create(ctx, cluster, createRequest)
	if createErr != nil {
		return apierror.InternalError(createErr)
	}

	response.Created(c)
	return nil
}
