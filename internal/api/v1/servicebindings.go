package v1

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/interfaces"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

// ServicebindingsController represents all functionality of the API related to service bindings
type ServicebindingsController struct {
}

// General behaviour: Internal errors (5xx) abort an action.
// Non-internal errors and warnings may be reported with it,
// however always after it. IOW an internal error is always
// the first element when reporting more than one error.

// Create handles the API endpoint /orgs/:org/applications/:app/servicebindings (POST)
// It creates a binding between the specified service and application
func (hc ServicebindingsController) Create(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")
	username, err := GetUsername(r)
	if err != nil {
		return UserNotFound()
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var bindRequest models.BindRequest
	err = json.Unmarshal(bodyBytes, &bindRequest)
	if err != nil {
		return BadRequest(err)
	}

	if len(bindRequest.Names) == 0 {
		err := errors.New("Cannot bind service without names")
		return BadRequest(err)
	}

	for _, serviceName := range bindRequest.Names {
		if serviceName == "" {
			err := errors.New("Cannot bind service with empty name")
			return BadRequest(err)
		}
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}
	if app == nil {
		return AppIsNotKnown(appName)
	}

	wl := application.NewWorkload(cluster, app.AppRef())

	// From here on out we collect errors and warnings per
	// service, to report as much as possible while also applying
	// as much as possible. IOW even when errors are reported it
	// is possible for some of the services to be properly bound.

	var theServices interfaces.ServiceList
	var theIssues []APIError

	for _, serviceName := range bindRequest.Names {
		service, err := services.Lookup(ctx, cluster, org, serviceName)
		if err != nil {
			if err.Error() == "service not found" {
				theIssues = append(theIssues, ServiceIsNotKnown(serviceName))
				continue
			}

			theIssues = append([]APIError{InternalError(err)}, theIssues...)
			return MultiError{theIssues}
		}

		theServices = append(theServices, service)
	}

	resp := models.BindResponse{}

	for _, service := range theServices {
		err = wl.Bind(ctx, service, username)
		if err != nil {
			if err.Error() == "service already bound" {
				resp.WasBound = append(resp.WasBound, service.Name())
				continue
			}

			theIssues = append([]APIError{InternalError(err)}, theIssues...)
			return MultiError{theIssues}
		}
	}

	if len(theIssues) > 0 {
		return MultiError{theIssues}
	}

	err = jsonResponse(w, resp)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Delete handles the API endpoint /orgs/:org/applications/:app/servicebindings/:service
// It removes the binding between the specified service and application
func (hc ServicebindingsController) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")
	serviceName := params.ByName("service")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}
	if app == nil {
		return AppIsNotKnown(appName)
	}

	wl := application.NewWorkload(cluster, app.AppRef())

	service, err := services.Lookup(ctx, cluster, org, serviceName)
	if err != nil && err.Error() == "service not found" {
		return ServiceIsNotKnown(serviceName)
	}
	if err != nil {
		return InternalError(err)
	}

	err = wl.Unbind(ctx, service)
	if err != nil && err.Error() == "service is not bound to the application" {
		return ServiceIsNotBound(serviceName)
	}
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte{})
	if err != nil {
		return InternalError(err)
	}

	return nil
}
