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

	client, clientError := cluster.ClientBuilderImage()
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	exists, existsError := builderimage.Exists(ctx, client, name)
	if existsError != nil {
		return apierror.InternalError(existsError)
	}
	if !exists {
		return apierror.BuilderImageIsNotKnown(name)
	}

	var updateRequest models.BuilderImageUpdateRequest
	bindError := c.BindJSON(&updateRequest)
	if bindError != nil {
		return apierror.NewBadRequestError(bindError.Error())
	}

	updateError := builderimage.Update(ctx, client, name, updateRequest)
	if updateError != nil {
		return apierror.InternalError(updateError)
	}

	response.OK(c)
	return nil
}
