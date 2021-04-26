package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

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
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
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
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	app, err := application.Lookup(cluster, org, appName)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if app == nil {
		http.Error(w, fmt.Sprintf("application '%s' not found", appName),
			http.StatusNotFound)
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

	gitea, err := clients.GetGiteaClient()
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
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	app, err := application.Lookup(cluster, org, appName)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if app == nil {
		http.Error(w, fmt.Sprintf("application '%s' not found", appName),
			http.StatusNotFound)
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
		http.Error(w, err.Error(), code)
		return true
	}
	return false
}
