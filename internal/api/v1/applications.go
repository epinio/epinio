package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/kubernetes"
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

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	apps, err := application.List(cluster, gitea.Client, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	js, err := json.Marshal(apps)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func (hc ApplicationsController) Show(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	app, err := application.Lookup(cluster, gitea.Client, org, appName)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if app == nil {
		handleError(w, errors.New("application not found"),
			http.StatusNotFound)
		return
	}

	js, err := json.Marshal(app)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func (hc ApplicationsController) Delete(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	app, err := application.Lookup(cluster, gitea.Client, org, appName)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if app == nil {
		handleError(w, errors.New("application not found"),
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

	err = app.Delete()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	// The command above removes the application's deployment.
	// This in turn deletes the associated replicaset, and pod, in
	// this order. The pod being gone thus indicates command
	// completion, and is therefore what we are waiting on below.

	err = cluster.WaitForPodBySelectorMissing(nil,
		deployments.WorkloadsDeploymentID,
		fmt.Sprintf("cloudfoundry.org/guid=%s.%s", org, appName),
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
	w.Write(js)
}

// Write the error to the response writer and return  true if there was an error
func handleError(w http.ResponseWriter, err error, code int) bool {
	if err != nil {
		http.Error(w, err.Error(), code)
		return true
	}
	return false
}
