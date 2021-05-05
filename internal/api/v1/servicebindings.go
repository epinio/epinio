package v1

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
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
		err := errors.New("Cannot bind service without a name")
		handleError(w, err, http.StatusBadRequest)
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

	service, err := services.Lookup(cluster, org, bindRequest.Name)
	if err != nil && err.Error() == "service not found" {
		err := errors.Errorf("service '%s' not found", bindRequest.Name)
		handleError(w, err, http.StatusNotFound)
		return
	}
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	err = app.Bind(service)
	if err != nil && err.Error() == "service already bound" {
		err := errors.Errorf("service '%s' already bound", bindRequest.Name)
		handleError(w, err, http.StatusConflict)
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

	service, err := services.Lookup(cluster, org, serviceName)
	if err != nil && err.Error() == "service not found" {
		err := errors.Errorf("service '%s' not found", serviceName)
		handleError(w, err, http.StatusNotFound)
		return
	}
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	err = app.Unbind(service)
	if err != nil && err.Error() == "service is not bound to the application" {
		err := errors.Errorf("service '%s' is not bound", serviceName)
		handleError(w, err, http.StatusBadRequest)
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
