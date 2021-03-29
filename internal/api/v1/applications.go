package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fatih/color"
	"github.com/julienschmidt/httprouter"
	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/internal/application"
	"github.com/suse/carrier/internal/cli/clients"
)

type ApplicationsController struct {
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	client, err := clients.NewCarrierClient(nil)
	if handleError(w, err, 500) {
		return
	}

	apps, err := application.List(client.KubeClient, client.GiteaClient, org)
	if handleError(w, err, 500) {
		return
	}

	result := []map[string]interface{}{}
	for _, app := range apps {
		status, err := client.KubeClient.DeploymentStatus(
			deployments.WorkloadsDeploymentID,
			fmt.Sprintf("app.kubernetes.io/part-of=%s,app.kubernetes.io/name=%s",
				org, app.Name))
		if handleError(w, err, 500) {
			return
		}

		routes, err := client.KubeClient.ListIngressRoutes(
			deployments.WorkloadsDeploymentID,
			app.Name)
		if err != nil {
			routes = []string{err.Error()}
		}

		var bonded = []string{}
		bound, err := app.Services()
		if err != nil {
			bonded = append(bonded, color.RedString(err.Error()))
		} else {
			for _, service := range bound {
				bonded = append(bonded, service.Name())
			}
		}
		result = append(result, map[string]interface{}{
			"name":          app.Name,
			"status":        status,
			"routes":        routes,
			"boundServices": bonded,
		})
	}

	js, err := json.Marshal(result)
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
