package application

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

const (
	DefaultInstances = int32(1)
	LocalRegistry    = "127.0.0.1:30500/apps"
)

// Deploy handles the API endpoint /namespaces/:namespace/applications/:app/deploy
// It uses an application chart to create the deployment, configuration and ingress (kube)
// resources for the app.
func (hc Controller) Deploy(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	name := c.Param("app")
	username := requestctx.User(ctx).Username

	req := models.DeployRequest{}
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequestError(err.Error()).WithDetails("failed to unmarshal deploy request")
	}

	if name != req.App.Name {
		return apierror.NewBadRequestError("name parameter from URL does not match name param in body")
	}
	if namespace != req.App.Namespace {
		return apierror.NewBadRequestError("namespace parameter from URL does not match namespace param in body")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	applicationCR, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown("cannot deploy app, application resource is missing")
		}
		return apierror.InternalError(err, "failed to get the application resource")
	}

	err = deploy.UpdateImageURL(ctx, cluster, applicationCR, req.ImageURL)
	if err != nil {
		return apierror.InternalError(err, "failed to set application's image url")
	}

	routes, apierr := deploy.DeployApp(ctx, cluster, req.App, username, req.Stage.ID, &req.Origin, nil)
	if apierr != nil {
		return apierr
	}

	response.OKReturn(c, models.DeployResponse{
		Routes: routes,
	})
	return nil
}
