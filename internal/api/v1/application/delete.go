package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint DELETE /namespaces/:org/applications/:app
// It removes the named application
func (hc Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	org := c.Param("org")
	appName := c.Param("app")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.OrgIsNotKnown(org)
	}

	app := models.NewAppRef(appName, org)

	found, err := application.Exists(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}
	if !found {
		return apierror.AppIsNotKnown(appName)
	}

	services, err := application.BoundServiceNames(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	resp := models.ApplicationDeleteResponse{
		UnboundServices: services,
	}

	err = application.Delete(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, resp)
	return nil
}
