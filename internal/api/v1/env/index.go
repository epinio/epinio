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

// Index handles the API endpoint /orgs/:org/applications/:app/environment
// It receives the org, application name and returns the environment
// associated with that application
func (hc Controller) Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := tracelog.Logger(ctx)

	orgName := c.Param("org")
	appName := c.Param("app")

	log.Info("returning environment", "org", orgName, "app", appName)

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

	app := models.NewAppRef(appName, orgName)

	exists, err = application.Exists(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.AppIsNotKnown(appName)
	}

	environment, err := application.Environment(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = response.JSON(c, environment)
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
