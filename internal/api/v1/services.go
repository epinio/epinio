package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
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
	if handleError(w, err, http.StatusInternalServerError) {
		return
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
	w.Write(js)
}

func (sc ServicesController) Index(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	epinioClient, err := clients.NewEpinioClient(nil)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
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
	appsOf, err := epinioClient.ServicesToApps(org)
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
	w.Write(js)
}
