package servicebinding

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"

	"github.com/pkg/errors"
)

// General behaviour: Internal errors (5xx) abort an action.
// Non-internal errors and warnings may be reported with it,
// however always after it. IOW an internal error is always
// the first element when reporting more than one error.

// Create handles the API endpoint /namespaces/:namespace/applications/:app/servicebindings (POST)
// It creates a binding between the specified service and application
func (hc Controller) Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	username := requestctx.User(ctx)

	var bindRequest models.BindRequest
	err := c.BindJSON(&bindRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	if len(bindRequest.Names) == 0 {
		err := errors.New("Cannot bind service without names")
		return apierror.BadRequest(err)
	}

	for _, serviceName := range bindRequest.Names {
		if serviceName == "" {
			err := errors.New("Cannot bind service with empty name")
			return apierror.BadRequest(err)
		}
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := namespaces.Exists(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}
	if !exists {
		return apierror.NamespaceIsNotKnown(namespace)
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	// Collect errors and warnings per service, to report as much
	// as possible while also applying as much as possible. IOW
	// even when errors are reported it is possible for some of
	// the services to be properly bound.

	// Take old state - See validation for use
	oldBound, err := application.BoundServiceNameSet(ctx, cluster, app.Meta)
	if err != nil {
		return apierror.InternalError(err)
	}

	resp := models.BindResponse{}

	var theIssues []apierror.APIError
	var okToBind []string

	// Validate existence of new services. Report invalid services as errors, later.
	// Filter out the services already bound, to be reported as regular response.
	for _, serviceName := range bindRequest.Names {
		if _, ok := oldBound[serviceName]; ok {
			resp.WasBound = append(resp.WasBound, serviceName)
			continue
		}

		_, err := services.Lookup(ctx, cluster, namespace, serviceName)
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

	if len(okToBind) > 0 {
		// Save those that were valid and not yet bound to the
		// application. Extends the set.

		err := application.BoundServicesSet(ctx, cluster, app.Meta, okToBind, false)
		if err != nil {
			theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
			return apierror.NewMultiError(theIssues)
		}

		// Update the workload, if there is any.
		if app.Workload != nil {
			_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "", nil, nil)
			if apierr != nil {
				return apierr
			}
		}
	}

	if len(theIssues) > 0 {
		return apierror.NewMultiError(theIssues)
	}

	response.OKReturn(c, resp)
	return nil
}
