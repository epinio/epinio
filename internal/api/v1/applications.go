package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

type ApplicationsController struct {
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) []APIError {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	if !exists {
		return []APIError{
			NewAPIError(fmt.Sprintf("Organization '%s' does not exist", org), "", http.StatusNotFound),
		}
	}

	apps, err := application.List(cluster, org)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	js, err := json.Marshal(apps)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return []APIError{}
}

func (hc ApplicationsController) Show(w http.ResponseWriter, r *http.Request) []APIError {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	if !exists {
		return []APIError{
			NewAPIError(fmt.Sprintf("Organization '%s' does not exist", org), "", http.StatusNotFound),
		}
	}

	app, err := application.Lookup(cluster, org, appName)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}
	if app == nil {
		return []APIError{
			NewAPIError(fmt.Sprintf("application '%s' not found", appName), "", http.StatusNotFound),
		}
	}

	js, err := json.Marshal(app)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return []APIError{}
}

func (hc ApplicationsController) Delete(w http.ResponseWriter, r *http.Request) []APIError {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	gitea, err := gitea.New()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	if !exists {
		return []APIError{
			NewAPIError(fmt.Sprintf("Organization '%s' does not exist", org), "", http.StatusNotFound),
		}
	}

	app, err := application.Lookup(cluster, org, appName)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	if app == nil {
		return []APIError{
			NewAPIError(fmt.Sprintf("application '%s' not found", appName), "", http.StatusNotFound),
		}
	}

	if len(app.BoundServices) > 0 {
		for _, bonded := range app.BoundServices {
			bound, err := services.Lookup(cluster, org, bonded)
			if err != nil {
				return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
			}

			err = app.Unbind(bound)
			if err != nil {
				return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
			}
		}
	}

	err = app.Delete(gitea)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
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
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	response := map[string][]string{}
	response["UnboundServices"] = app.BoundServices

	js, err := json.Marshal(response)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return []APIError{}
}
