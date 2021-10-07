package env

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/julienschmidt/httprouter"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Unset handles the API endpoint /orgs/:org/applications/:app/environment/:env (DELETE)
// It receives the org, application name, var name, and removes the
// variable from the application's environment.
func (hc Controller) Unset(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	orgName := params.ByName("org")
	appName := params.ByName("app")
	varName := params.ByName("env")

	log.Info("processing environment variable removal",
		"org", orgName, "app", appName, "var", varName)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, orgName)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return OrgIsNotKnown(orgName)
	}

	app, err := application.Lookup(ctx, cluster, orgName, appName)
	if err != nil {
		return InternalError(err)
	}
	if app == nil {
		return AppIsNotKnown(appName)
	}

	err = application.EnvironmentUnset(ctx, cluster, app.Meta, varName)
	if err != nil {
		return InternalError(err)
	}

	if app.Workload != nil {
		varNames, err := application.EnvironmentNames(ctx, cluster, app.Meta)
		if err != nil {
			return InternalError(err)
		}

		err = application.NewWorkload(cluster, app.Meta).EnvironmentChange(ctx, varNames)
		if err != nil {
			return InternalError(err)
		}
	}

	err = response.JSON(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
