package builderimage

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/builderimage"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Delete handles DELETE /builderimages/:name
func Delete(c *gin.Context) apierror.APIErrors {
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

	deleteError := builderimage.Delete(ctx, client, name)
	if deleteError != nil {
		return apierror.InternalError(deleteError)
	}

	response.OK(c)
	return nil
}
