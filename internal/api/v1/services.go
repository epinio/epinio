package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

type ServicesController struct {
}

func (sc ServicesController) Show(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	serviceName := params.ByName("service")

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

	service, err := services.Lookup(cluster, org, serviceName)
	if err != nil {
		if err.Error() == "service not found" {
			http.Error(w, fmt.Sprintf("Service '%s' does not exist", serviceName),
				http.StatusNotFound)
			return
		}
		if handleError(w, err, http.StatusInternalServerError) {
			return
		}
	}

	status, err := service.Status()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	serviceDetails, err := service.Details()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	responseData := map[string]string{
		"Status": status,
	}
	for key, value := range serviceDetails {
		responseData[key] = value
	}

	js, err := json.Marshal(responseData)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (sc ServicesController) Index(w http.ResponseWriter, r *http.Request) {
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

	orgServices, err := services.List(cluster, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	appsOf, err := servicesToApps(cluster, gitea, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	responseData := map[string]interface{}{
		"Services":    orgServices,
		"ServiceApps": appsOf,
	}

	js, err := json.Marshal(responseData)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func servicesToApps(cluster *kubernetes.Cluster, gitea *clients.GiteaClient, org string) (map[string]application.ApplicationList, error) {
	// Determine apps bound to services
	// (inversion of services bound to apps)
	// Literally query apps in the org for their services and invert.

	var appsOf = map[string]application.ApplicationList{}

	apps, err := application.List(cluster, gitea.Client, org)
	if err != nil {
		return nil, err
	}

	for _, app := range apps {
		bound, err := app.Services()
		if err != nil {
			return nil, err
		}
		for _, bonded := range bound {
			bname := bonded.Name()
			if theapps, found := appsOf[bname]; found {
				appsOf[bname] = append(theapps, app)
			} else {
				appsOf[bname] = application.ApplicationList{app}
			}
		}
	}

	return appsOf, nil
}
