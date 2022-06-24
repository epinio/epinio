package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint DELETE /namespaces/:namespace/applications/:app
// It removes the named application
func (hc Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	app := models.NewAppRef(appName, namespace)

	found, err := application.Exists(ctx, cluster, app)
	if err != nil {
		return apierror.NewInternalError(err)
	}
	if !found {
		return apierror.AppIsNotKnown(appName)
	}

	configurations, err := application.BoundConfigurationNames(ctx, cluster, app)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	resp := models.ApplicationDeleteResponse{
		UnboundConfigurations: configurations,
	}

	err = application.Delete(ctx, cluster, app)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	response.OKReturn(c, resp)
	return nil
}
