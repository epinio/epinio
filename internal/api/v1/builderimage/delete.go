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

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, existsErr := builderimage.Exists(ctx, cluster, name)
	if existsErr != nil {
		return apierror.InternalError(existsErr)
	}
	if !exists {
		return apierror.BuilderImageIsNotKnown(name)
	}

	deleteErr := builderimage.Delete(ctx, cluster, name)
	if deleteErr != nil {
		return apierror.InternalError(deleteErr)
	}

	response.OK(c)
	return nil
}
