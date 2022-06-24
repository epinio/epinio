package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/duration"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Running handles the API endpoint GET /namespaces/:namespace/applications/:app/running
// It waits for the specified application to be running (i.e. its
// deployment to be complete), before it returns. An exception is if
// the application does not become running without
// `duration.ToAppBuilt()` (default: 10 minutes). In that case it
// returns with an error after that time.
func (hc Controller) Running(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	if app.Workload == nil {
		// While the app exists it has no workload, and therefore no status
		return apierror.NewBadRequest("No status available for application without workload")
	}

	err = cluster.WaitForDeploymentCompleted(
		ctx, nil, namespace, app.Workload.Name, duration.ToAppBuilt())
	if err != nil {
		return apierror.NewInternalError(err)
	}

	response.OK(c)
	return nil
}
