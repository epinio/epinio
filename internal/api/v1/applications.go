package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/suse/carrier/internal/application"
	"github.com/suse/carrier/internal/cli/clients"
	"github.com/suse/carrier/kubernetes"
)

type ApplicationsController struct {
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, 500) {
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, 500) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, 500) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org), 404)
		return
	}

	apps, err := application.List(cluster, gitea.Client, org)
	if handleError(w, err, 500) {
		return
	}

	js, err := json.Marshal(apps)
	if handleError(w, err, 500) {
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
	if handleError(w, err, 500) {
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, 500) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, 500) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org), 404)
		return
	}

	app, err := application.Lookup(cluster, gitea.Client, org, appName)
	if handleError(w, err, 500) {
		return
	}

	js, err := json.Marshal(app)
	if handleError(w, err, 500) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// Write the error to the response writer and return  true if there was an error
func handleError(w http.ResponseWriter, err error, code int) bool {
	if err != nil {
		http.Error(w, err.Error(), 500)
		return true
	}
	return false
}
