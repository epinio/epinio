package v1

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
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

	// Collect errors and warnings per service, to report as much
	// as possible while also applying as much as possible. IOW
	// even when errors are reported it is possible for some of
	// the services to be properly bound.

	// Take old state
	oldBound, err := application.BoundServiceNameSet(ctx, cluster, app.Meta)
	if err != nil {
		return InternalError(err)
	}

	resp := models.BindResponse{}

	var theIssues []APIError
	var okToBind []string

	// Validate existence of new services. Report invalid services as errors, later.
	// Filter out the services already bound, to be reported as regular response.
	for _, serviceName := range bindRequest.Names {
		if _, ok := oldBound[serviceName]; ok {
			resp.WasBound = append(resp.WasBound, serviceName)
			continue
		}

		_, err := services.Lookup(ctx, cluster, org, serviceName)
		if err != nil {
			if err.Error() == "service not found" {
				theIssues = append(theIssues, ServiceIsNotKnown(serviceName))
				continue
			}

			theIssues = append([]APIError{InternalError(err)}, theIssues...)
			return MultiError{theIssues}
		}

		okToBind = append(okToBind, serviceName)
	}

	if len(okToBind) > 0 {
		// Save those that were valid and not yet bound to the
		// application. Extends the set.

		err := application.BoundServicesSet(ctx, cluster, app.Meta, okToBind, false)
		if err != nil {
			theIssues = append([]APIError{InternalError(err)}, theIssues...)
			return MultiError{theIssues}
		}

		// Update the workload, if there is any.
		if app.Workload != nil {
			// For this read the new set of bound services back,
			// as full service structures
			newBound, err := application.BoundServices(ctx, cluster, app.Meta)
			if err != nil {
				theIssues = append([]APIError{InternalError(err)}, theIssues...)
				return MultiError{theIssues}
			}

			err = application.NewWorkload(cluster, app.Meta).
				BoundServicesChange(ctx, username, oldBound, newBound)
			if err != nil {
				theIssues = append([]APIError{InternalError(err)}, theIssues...)
				return MultiError{theIssues}
			}
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
	username, err := GetUsername(r)
	if err != nil {
		return UserNotFound()
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

	apiErr := DeleteBinding(ctx, cluster, org, appName, serviceName, username)
	if apiErr != nil {
		return apiErr
	}

	err = jsonResponse(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func DeleteBinding(ctx context.Context, cluster *kubernetes.Cluster, org, appName, serviceName, username string) APIErrors {

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}
	if app == nil {
		return AppIsNotKnown(appName)
	}

	_, err = services.Lookup(ctx, cluster, org, serviceName)
	if err != nil && err.Error() == "service not found" {
		return ServiceIsNotKnown(serviceName)
	}
	if err != nil {
		return InternalError(err)
	}

	// Take old state
	oldBound, err := application.BoundServiceNameSet(ctx, cluster, app.Meta)
	if err != nil {
		return InternalError(err)
	}

	err = application.BoundServicesUnset(ctx, cluster, app.Meta, serviceName)
	if err != nil {
		return InternalError(err)
	}

	if app.Workload != nil {
		// For this read the new set of bound services back,
		// as full service structures
		newBound, err := application.BoundServices(ctx, cluster, app.Meta)
		if err != nil {
			return InternalError(err)
		}

		err = application.NewWorkload(cluster, app.Meta).BoundServicesChange(ctx, username, oldBound, newBound)
		if err != nil {
			return InternalError(err)
		}
	}

	return nil
}
