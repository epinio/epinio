package builderimage

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/builderimage"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	models "github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Update handles PATCH /builderimages/:name
func Update(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	name := c.Param("name")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	exists, existsErr := builderimage.Exists(ctx, cluster, name)
	if existsErr != nil {
		return apierror.InternalError(existsErr)
	}
	if !exists {
		return apierror.BuilderImageIsNotKnown(name)
	}

	var updateRequest models.BuilderImageUpdateRequest
	if bindErr := c.BindJSON(&updateRequest); bindErr != nil {
		return apierror.NewBadRequestError(bindErr.Error())
	}

	updateErr := builderimage.Update(ctx, cluster, name, updateRequest)
	if updateErr != nil {
		return apierror.InternalError(updateErr)
	}

	response.OK(c)
	return nil
}
