package env

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/julienschmidt/httprouter"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Controller represents all functionality of the API related to envs
type Controller struct{}

// Index handles the API endpoint /orgs/:org/applications/:app/environment
// It receives the org, application name and returns the environment
// associated with that application
func (hc Controller) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	orgName := params.ByName("org")
	appName := params.ByName("app")

	log.Info("returning environment", "org", orgName, "app", appName)

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

	app := models.NewAppRef(appName, orgName)

	exists, err = application.Exists(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	environment, err := application.Environment(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	err = response.JSON(w, environment)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Match handles the API endpoint /orgs/:org/applications/:app/environment/:env/match/:pattern
// It receives the org, application name, plus a prefix and returns
// the names of all the environment associated with that application
// with prefix
func (hc Controller) Match(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	orgName := params.ByName("org")
	appName := params.ByName("app")
	prefix := params.ByName("pattern")

	log.Info("returning matching environment variable names",
		"org", orgName, "app", appName, "prefix", prefix)

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

	app := models.NewAppRef(appName, orgName)

	exists, err = application.Exists(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	// EnvList, with post-processing - selection of matches, and
	// projection to deliver only names

	environment, err := application.Environment(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	matches := []string{}
	for _, ev := range environment {
		if strings.HasPrefix(ev.Name, prefix) {
			matches = append(matches, ev.Name)
		}
	}

	err = response.JSON(w, models.EnvMatchResponse{Names: matches})
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Set handles the API endpoint /orgs/:org/applications/:app/environment (POST)
// It receives the org, application name, var name and value,
// and add/modifies the variable in the  application's environment.
func (hc Controller) Set(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	orgName := params.ByName("org")
	appName := params.ByName("app")

	log.Info("processing environment variable assignment",
		"org", orgName, "app", appName)

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

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var setRequest models.EnvVariableList
	err = json.Unmarshal(bodyBytes, &setRequest)
	if err != nil {
		return BadRequest(err)
	}

	err = application.EnvironmentSet(ctx, cluster, app.Meta, setRequest, false)
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

// EnvShow handles the API endpoint /orgs/:org/applications/:app/environment/:env
// It receives the org, application name, var name, and returns
// the variable's value in the application's environment.
func (hc Controller) Show(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	orgName := params.ByName("org")
	appName := params.ByName("app")
	varName := params.ByName("env")

	log.Info("processing environment variable request",
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

	app := models.NewAppRef(appName, orgName)

	exists, err = application.Exists(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	// EnvList, with post-processing - select specific value

	environment, err := application.Environment(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	var match models.EnvVariable
	for _, ev := range environment {
		if ev.Name == varName {
			match = ev
			break
		}
	}

	// Not found => Returns a nil object

	err = response.JSON(w, match)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

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
