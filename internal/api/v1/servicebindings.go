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
	"github.com/pkg/errors"
)

type ServicebindingsController struct {
}

func (hc ServicebindingsController) Create(w http.ResponseWriter, r *http.Request) []APIError {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	var bindRequest models.BindRequest
	err = json.Unmarshal(bodyBytes, &bindRequest)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusBadRequest)}
	}

	if bindRequest.Name == "" {
		err := errors.New("Cannot bind service without a name")
		if err != nil {
			return []APIError{NewAPIError(err.Error(), "", http.StatusBadRequest)}
		}
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

	service, err := services.Lookup(cluster, org, bindRequest.Name)
	if err != nil && err.Error() == "service not found" {
		return []APIError{
			NewAPIError(fmt.Sprintf("service '%s' not found", bindRequest.Name), "", http.StatusNotFound),
		}
	}
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	err = app.Bind(service)
	if err != nil && err.Error() == "service already bound" {
		return []APIError{
			NewAPIError(fmt.Sprintf("service '%s' already bound", bindRequest.Name), "", http.StatusConflict),
		}
	}
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte{})
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return []APIError{}
}

func (hc ServicebindingsController) Delete(w http.ResponseWriter, r *http.Request) []APIError {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")
	serviceName := params.ByName("service")

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

	service, err := services.Lookup(cluster, org, serviceName)
	if err != nil && err.Error() == "service not found" {
		return []APIError{
			NewAPIError(fmt.Sprintf("service '%s' not found", serviceName), "", http.StatusNotFound),
		}
	}
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	err = app.Unbind(service)
	if err != nil && err.Error() == "service is not bound to the application" {
		return []APIError{
			NewAPIError(fmt.Sprintf("service '%s' is not bound", serviceName), "", http.StatusBadRequest),
		}
	}
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte{})
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return []APIError{}
}
