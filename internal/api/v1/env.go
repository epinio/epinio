package v1

import (
	"net/http"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/julienschmidt/httprouter"
)

// EnvIndex receives the org, application name and returns the
// environment associated with that application
func (hc ApplicationsController) EnvIndex(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	app := params.ByName("app")

	log.Info("returning environment", "org", org, "app", app)

	return nil
}

// EnvMatch receives the org, application name, plus a prefix and
// returns the names of all the environment associated with that
// application with prefix
func (hc ApplicationsController) EnvMatch(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	app := params.ByName("app")
	prefix := params.ByName("pattern")

	log.Info("returning matching environment variable names",
		"org", org, "app", app, "prefix", prefix)

	return nil
}

// EnvSet receives the org, application name, var name and value,
// and add/modifies the variable in the  application's environment.
func (hc ApplicationsController) EnvSet(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	app := params.ByName("app")

	log.Info("processing environment variable assignment",
		"org", org, "app", app)

	return nil
}

// EnvShow receives the org, application name, var name, and returns
// the variable's value in the application's environment.
func (hc ApplicationsController) EnvShow(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	app := params.ByName("app")
	varname := params.ByName("env")

	log.Info("processing environment variable request",
		"org", org, "app", app, "var", varname)

	return nil
}

// EnvUnset receives the org, application name, var name, and removes
// the variable from the application's environment.
func (hc ApplicationsController) EnvUnset(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	app := params.ByName("app")
	varname := params.ByName("env")

	log.Info("processing environment variable removal",
		"org", org, "app", app, "var", varname)

	return nil
}
