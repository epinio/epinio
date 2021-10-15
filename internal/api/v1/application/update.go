package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Update handles the API endpoint PATCH /namespaces/:org/applications/:app
// It modifies the specified application. Currently this is only the
// number of instances to run.
func (hc Controller) Update(c *gin.Context) apierror.APIErrors { // nolint:gocyclo // simplification defered
	ctx := c.Request.Context()
	org := c.Param("org")
	appName := c.Param("app")
	username := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.OrgIsNotKnown(org)
	}

	exists, err = application.Exists(ctx, cluster, models.NewAppRef(appName, org))
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.AppIsNotKnown(appName)
	}

	// Retrieve and validate update request ...

	var updateRequest models.ApplicationUpdateRequest
	err = c.BindJSON(&updateRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	if updateRequest.Instances != nil && *updateRequest.Instances < 0 {
		return apierror.NewBadRequest("instances param should be integer equal or greater than zero")
	}

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	// TODO: Can we optimize to perform a single restart regardless of what changed ?!
	// TODO: Should we ?

	if updateRequest.Instances != nil {
		desired := *updateRequest.Instances

		// Save to configuration
		err := application.ScalingSet(ctx, cluster, app.Meta, desired)
		if err != nil {
			return apierror.InternalError(err)
		}

		// Restart workload, if any
		if app.Workload != nil {
			err = application.NewWorkload(cluster, app.Meta).Scale(ctx, desired)
			if err != nil {
				return apierror.InternalError(err)
			}
		}
	}

	if len(updateRequest.Environment) > 0 {
		err := application.EnvironmentSet(ctx, cluster, app.Meta, updateRequest.Environment, true)
		if err != nil {
			return apierror.InternalError(err)
		}

		// Restart workload, if any
		if app.Workload != nil {
			// For this read the new set of variables back
			varNames, err := application.EnvironmentNames(ctx, cluster, app.Meta)
			if err != nil {
				return apierror.InternalError(err)
			}

			err = application.NewWorkload(cluster, app.Meta).
				EnvironmentChange(ctx, varNames)
			if err != nil {
				return apierror.InternalError(err)
			}
		}
	}

	if len(updateRequest.Services) > 0 {
		// Take old state
		oldBound, err := application.BoundServiceNameSet(ctx, cluster, app.Meta)
		if err != nil {
			return apierror.InternalError(err)
		}

		var theIssues []apierror.APIError
		var okToBind []string

		for _, serviceName := range updateRequest.Services {
			_, err := services.Lookup(ctx, cluster, org, serviceName)
			if err != nil {
				if err.Error() == "service not found" {
					theIssues = append(theIssues, apierror.ServiceIsNotKnown(serviceName))
					continue
				}

				theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
				return apierror.NewMultiError(theIssues)
			}

			okToBind = append(okToBind, serviceName)
		}

		err = application.BoundServicesSet(ctx, cluster, app.Meta, okToBind, true)
		if err != nil {
			theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
			return apierror.NewMultiError(theIssues)
		}

		// Restart workload, if any
		if app.Workload != nil {
			// For this read the new set of bound services back,
			// as full service structures
			newBound, err := application.BoundServices(ctx, cluster, app.Meta)
			if err != nil {
				theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
				return apierror.NewMultiError(theIssues)
			}

			err = application.NewWorkload(cluster, app.Meta).
				BoundServicesChange(ctx, username, oldBound, newBound)
			if err != nil {
				theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
				return apierror.NewMultiError(theIssues)
			}
		}

		if len(theIssues) > 0 {
			return apierror.NewMultiError(theIssues)
		}
	}

	response.OK(c)
	return nil
}
