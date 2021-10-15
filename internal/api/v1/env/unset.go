package env

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Unset handles the API endpoint /orgs/:org/applications/:app/environment/:env (DELETE)
// It receives the org, application name, var name, and removes the
// variable from the application's environment.
func (hc Controller) Unset(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := tracelog.Logger(ctx)

	orgName := c.Param("org")
	appName := c.Param("app")
	varName := c.Param("env")

	log.Info("processing environment variable removal",
		"org", orgName, "app", appName, "var", varName)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, orgName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.OrgIsNotKnown(orgName)
	}

	app, err := application.Lookup(ctx, cluster, orgName, appName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	err = application.EnvironmentUnset(ctx, cluster, app.Meta, varName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app.Workload != nil {
		varNames, err := application.EnvironmentNames(ctx, cluster, app.Meta)
		if err != nil {
			return apierror.InternalError(err)
		}

		err = application.NewWorkload(cluster, app.Meta).EnvironmentChange(ctx, varNames)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	response.OK(c)
	return nil
}
