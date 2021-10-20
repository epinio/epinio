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

// Create handles the API endpoint POST /namespaces/:org/applications
// It creates a new and empty application. I.e. without a workload.
func (hc Controller) Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("org")
	username := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.OrgIsNotKnown(namespace)
	}

	var createRequest models.ApplicationCreateRequest
	err = c.BindJSON(&createRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	appRef := models.NewAppRef(createRequest.Name, namespace)
	found, err := application.Exists(ctx, cluster, appRef)
	if err != nil {
		return apierror.InternalError(err, "failed to check for app resource")
	}
	if found {
		return apierror.AppAlreadyKnown(createRequest.Name)
	}

	// Sanity check the services, if any. IOW anything to be bound
	// has to exist now.  We will check again when the application
	// is deployed, to guard against bound services being removed
	// from now till then. While it should not be possible through
	// epinio itself (*), external editing of the relevant
	// resources cannot be excluded from consideration.
	//
	// (*) `epinio service delete S` on a bound service S will
	//      either reject the operation, or, when forced, unbind S
	//      from the app.

	var theIssues []apierror.APIError

	for _, serviceName := range createRequest.Configuration.Services {
		_, err := services.Lookup(ctx, cluster, namespace, serviceName)
		if err != nil {
			if err.Error() == "service not found" {
				theIssues = append(theIssues, apierror.ServiceIsNotKnown(serviceName))
				continue
			}

			theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
			return apierror.NewMultiError(theIssues)
		}
	}

	if len(theIssues) > 0 {
		return apierror.NewMultiError(theIssues)
	}

	// Arguments found OK, now we can modify the system state

	err = application.Create(ctx, cluster, appRef, username)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Instances
	desired := DefaultInstances
	if createRequest.Configuration.Instances != nil {
		desired = *createRequest.Configuration.Instances
	}

	err = application.ScalingSet(ctx, cluster, appRef, desired)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Probes
	if createRequest.Configuration.Health != nil {
		if createRequest.Configuration.Health.Live != nil {
			err = application.LivenessSet(ctx, cluster, appRef,
				createRequest.Configuration.Health.Live)
			if err != nil {
				return apierror.InternalError(err)
			}
		}

		if createRequest.Configuration.Health.Ready != nil {
			err = application.ReadinessSet(ctx, cluster, appRef,
				createRequest.Configuration.Health.Ready)
			if err != nil {
				return apierror.InternalError(err)
			}
		}
	}

	// Save service information.
	err = application.BoundServicesSet(ctx, cluster, appRef,
		createRequest.Configuration.Services, true)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Save environment assignments
	err = application.EnvironmentSet(ctx, cluster, appRef,
		createRequest.Configuration.Environment, true)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}
