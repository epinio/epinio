package application

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Running handles the API endpoint GET /namespaces/:org/applications/:app/running
// It waits for the specified application to be running (i.e. its
// deployment to be complete), before it returns. An exception is if
// the application does not become running without
// `duration.ToAppBuilt()` (default: 10 minutes). In that case it
// returns with an error after that time.
func (hc Controller) Running(c *gin.Context) apierror.APIErrors {
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

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	if app.Workload == nil {
		// While the app exists it has no workload, and therefore no status
		return apierror.NewAPIError("No status available for application without workload",
			"", http.StatusBadRequest)
	}

	err = cluster.WaitForDeploymentCompleted(
		ctx, nil, org, appName, duration.ToAppBuilt())
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}
