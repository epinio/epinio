package application

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Restart handles the API endpoint POST /namespaces/:namespace/applications/:app/restart
func (hc Controller) Restart(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	//username := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := namespaces.Exists(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.NamespaceIsNotKnown(namespace)
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	if app.Workload == nil {
		return apierror.NewAPIError("No restart possible for an application without workload",
			"", http.StatusBadRequest)
	}

	err = application.NewWorkload(cluster, app.Meta).Restart(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}
