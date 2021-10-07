package application

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/julienschmidt/httprouter"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Update handles the API endpoint PATCH /namespaces/:org/applications/:app
// It modifies the specified application. Currently this is only the
// number of instances to run.
func (hc Controller) Update(w http.ResponseWriter, r *http.Request) APIErrors { // nolint:gocyclo // simplification defered
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")
	username := requestctx.User(ctx)

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

	exists, err = application.Exists(ctx, cluster, models.NewAppRef(appName, org))
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	// Retrieve and validate update request ...

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var updateRequest models.ApplicationUpdateRequest
	err = json.Unmarshal(bodyBytes, &updateRequest)
	if err != nil {
		return BadRequest(err)
	}

	if updateRequest.Instances != nil && *updateRequest.Instances < 0 {
		return NewBadRequest("instances param should be integer equal or greater than zero")
	}

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}

	// TODO: Can we optimize to perform a single restart regardless of what changed ?!
	// TODO: Should we ?

	if updateRequest.Instances != nil {
		desired := *updateRequest.Instances

		// Save to configuration
		err := application.ScalingSet(ctx, cluster, app.Meta, desired)
		if err != nil {
			return InternalError(err)
		}

		// Restart workload, if any
		if app.Workload != nil {
			err = application.NewWorkload(cluster, app.Meta).Scale(ctx, desired)
			if err != nil {
				return InternalError(err)
			}
		}
	}

	if len(updateRequest.Environment) > 0 {
		err := application.EnvironmentSet(ctx, cluster, app.Meta, updateRequest.Environment, true)
		if err != nil {
			return InternalError(err)
		}

		// Restart workload, if any
		if app.Workload != nil {
			// For this read the new set of variables back
			varNames, err := application.EnvironmentNames(ctx, cluster, app.Meta)
			if err != nil {
				return InternalError(err)
			}

			err = application.NewWorkload(cluster, app.Meta).
				EnvironmentChange(ctx, varNames)
			if err != nil {
				return InternalError(err)
			}
		}
	}

	if len(updateRequest.Services) > 0 {
		// Take old state
		oldBound, err := application.BoundServiceNameSet(ctx, cluster, app.Meta)
		if err != nil {
			return InternalError(err)
		}

		var theIssues []APIError
		var okToBind []string

		for _, serviceName := range updateRequest.Services {
			_, err := services.Lookup(ctx, cluster, org, serviceName)
			if err != nil {
				if err.Error() == "service not found" {
					theIssues = append(theIssues, ServiceIsNotKnown(serviceName))
					continue
				}

				theIssues = append([]APIError{InternalError(err)}, theIssues...)
				return NewMultiError(theIssues)
			}

			okToBind = append(okToBind, serviceName)
		}

		err = application.BoundServicesSet(ctx, cluster, app.Meta, okToBind, true)
		if err != nil {
			theIssues = append([]APIError{InternalError(err)}, theIssues...)
			return NewMultiError(theIssues)
		}

		// Restart workload, if any
		if app.Workload != nil {
			// For this read the new set of bound services back,
			// as full service structures
			newBound, err := application.BoundServices(ctx, cluster, app.Meta)
			if err != nil {
				theIssues = append([]APIError{InternalError(err)}, theIssues...)
				return NewMultiError(theIssues)
			}

			err = application.NewWorkload(cluster, app.Meta).
				BoundServicesChange(ctx, username, oldBound, newBound)
			if err != nil {
				theIssues = append([]APIError{InternalError(err)}, theIssues...)
				return NewMultiError(theIssues)
			}
		}

		if len(theIssues) > 0 {
			return NewMultiError(theIssues)
		}
	}

	err = response.JSON(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
