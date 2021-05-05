package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

// APIError struct is meant to host an error as described here:
// https://jsonapi.org/examples/#error-objects-basics
type APIError map[string]([]map[string]string)

type ApplicationsController struct {
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := organizations.Exists(cluster, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		err := errors.Errorf("Organization '%s' does not exist", org)
		handleError(w, err, http.StatusNotFound)
		return
	}

	apps, err := application.List(cluster, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	js, err := json.Marshal(apps)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (hc ApplicationsController) Show(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := organizations.Exists(cluster, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		err := errors.Errorf("Organization '%s' does not exist", org)
		handleError(w, err, http.StatusNotFound)
		return
	}

	app, err := application.Lookup(cluster, org, appName)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if app == nil {
		err := errors.Errorf("application '%s' not found", appName)
		handleError(w, err, http.StatusNotFound)
		return
	}

	js, err := json.Marshal(app)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (hc ApplicationsController) Delete(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	gitea, err := gitea.New()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := organizations.Exists(cluster, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		err := errors.Errorf("Organization '%s' does not exist", org)
		handleError(w, err, http.StatusNotFound)
		return
	}

	app, err := application.Lookup(cluster, org, appName)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if app == nil {
		err := errors.Errorf("application '%s' not found", appName)
		handleError(w, err, http.StatusNotFound)
		return
	}

	if len(app.BoundServices) > 0 {
		for _, bonded := range app.BoundServices {
			bound, err := services.Lookup(cluster, org, bonded)
			if handleError(w, err, http.StatusInternalServerError) {
				return
			}

			err = app.Unbind(bound)
			if handleError(w, err, http.StatusInternalServerError) {
				return
			}
		}
	}

	err = app.Delete(gitea)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	// The command above removes the application's deployment.
	// This in turn deletes the associated replicaset, and pod, in
	// this order. The pod being gone thus indicates command
	// completion, and is therefore what we are waiting on below.

	// TODO: Implement a WaitForDeletion on the Application
	err = cluster.WaitForPodBySelectorMissing(nil,
		app.Organization,
		fmt.Sprintf("app.kubernetes.io/name=%s", appName),
		duration.ToDeployment())
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	response := map[string][]string{}
	response["UnboundServices"] = app.BoundServices

	js, err := json.Marshal(response)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

// Write the error to the response writer and return  true if there was an error
func handleError(w http.ResponseWriter, err error, code int) bool {
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		response := APIError{
			"errors": {
				{
					"status": strconv.Itoa(code),
					"title":  err.Error(),
				},
			},
		}

		js, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, marshalErr.Error())
			return true
		}

		w.WriteHeader(code)
		fmt.Fprintln(w, string(js))

		return true
	}
	return false
}
