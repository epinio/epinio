package env

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Set handles the API endpoint /orgs/:org/applications/:app/environment (POST)
// It receives the org, application name, var name and value,
// and add/modifies the variable in the  application's environment.
func (hc Controller) Set(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := tracelog.Logger(ctx)

	orgName := c.Param("org")
	appName := c.Param("app")

	log.Info("processing environment variable assignment",
		"org", orgName, "app", appName)

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

	var setRequest models.EnvVariableMap
	err = c.BindJSON(&setRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	err = application.EnvironmentSet(ctx, cluster, app.Meta, setRequest, false)
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

	err = response.JSON(c, models.ResponseOK)
	if err != nil {
		return apierror.InternalError(err)
	}
	return nil
}
