package v1

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

type ServicebindingsController struct {
}

func (hc ServicebindingsController) Create(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	var bindRequest models.BindRequest
	err = json.Unmarshal(bodyBytes, &bindRequest)
	if handleError(w, err, http.StatusBadRequest) {
		return
	}

	if bindRequest.Name == "" {
		http.Error(w, fmt.Sprintf("Cannot bind service without a name"),
			http.StatusBadRequest)
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

	service, err := services.Lookup(cluster, org, bindRequest.Name)
	if err != nil && err.Error() == "service not found" {
		http.Error(w, fmt.Sprintf("service '%s' not found", bindRequest.Name),
			http.StatusNotFound)
		return
	}
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	err = app.Bind(service)
	if err != nil && err.Error() == "service already bound" {
		http.Error(w, fmt.Sprintf("service '%s' already bound", bindRequest.Name),
			http.StatusConflict)
		return
	}
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte{})
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (hc ServicebindingsController) Delete(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")
	serviceName := params.ByName("service")

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

	service, err := services.Lookup(cluster, org, serviceName)
	if err != nil && err.Error() == "service not found" {
		http.Error(w, fmt.Sprintf("service '%s' not found", serviceName),
			http.StatusNotFound)
		return
	}
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	err = app.Unbind(service)
	if err != nil && err.Error() == "service is not bound to the application" {
		http.Error(w, fmt.Sprintf("service '%s' is not bound", serviceName),
			http.StatusBadRequest)
		return
	}
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte{})
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}
