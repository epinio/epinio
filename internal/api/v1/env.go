package v1

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/julienschmidt/httprouter"
)

// EnvIndex receives the org, application name and returns the
// environment associated with that application
func (hc ApplicationsController) EnvIndex(w http.ResponseWriter, r *http.Request) APIErrors {
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

	js, err := json.Marshal(environment)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// EnvMatch receives the org, application name, plus a prefix and
// returns the names of all the environment associated with that
// application with prefix
func (hc ApplicationsController) EnvMatch(w http.ResponseWriter, r *http.Request) APIErrors {
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

	js, err := json.Marshal(matches)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// EnvSet receives the org, application name, var name and value,
// and add/modifies the variable in the  application's environment.
func (hc ApplicationsController) EnvSet(w http.ResponseWriter, r *http.Request) APIErrors {
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

	app := models.NewAppRef(appName, orgName)

	exists, err = application.Exists(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
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

	err = application.EnvironmentSet(ctx, cluster, app, setRequest)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// EnvShow receives the org, application name, var name, and returns
// the variable's value in the application's environment.
func (hc ApplicationsController) EnvShow(w http.ResponseWriter, r *http.Request) APIErrors {
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

	var match *models.EnvVariable
	for _, ev := range environment {
		if ev.Name == varName {
			match = &ev
			break
		}
	}

	// Not found => Returns a nil object

	js, err := json.Marshal(match)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// EnvUnset receives the org, application name, var name, and removes
// the variable from the application's environment.
func (hc ApplicationsController) EnvUnset(w http.ResponseWriter, r *http.Request) APIErrors {
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

	app := models.NewAppRef(appName, orgName)

	exists, err = application.Exists(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	err = application.EnvironmentUnset(ctx, cluster, app, varName)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
