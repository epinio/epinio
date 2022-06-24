package application

import (
	"fmt"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Update handles the API endpoint PATCH /namespaces/:namespace/applications/:app
func (hc Controller) Update(c *gin.Context) apierror.APIErrors { // nolint:gocyclo // simplification defered
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	username := requestctx.User(ctx).Username

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	appRef := models.NewAppRef(appName, namespace)
	exists, err := application.Exists(ctx, cluster, appRef)
	if err != nil {
		return apierror.NewInternalError(err)
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

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	// Check if the request contains any changes. Abort early if not.

	// if there is nothing to change
	if updateRequest.Instances == nil &&
		len(updateRequest.Environment) == 0 &&
		updateRequest.Configurations == nil &&
		len(updateRequest.Routes) == 0 &&
		updateRequest.AppChart == "" {
		response.OK(c)
		return nil
	}

	// Save all changes to the relevant parts of the app resources (CRD, secrets, and the like).

	if updateRequest.AppChart != "" && updateRequest.AppChart != app.Configuration.AppChart {
		if app.Workload != nil {
			return apierror.NewBadRequest("Unable to change app chart of active application")
		}

		found, err := appchart.Exists(ctx, cluster, updateRequest.AppChart)
		if err != nil {
			return apierror.NewInternalError(err)
		}
		if !found {
			return apierror.AppChartIsNotKnown(updateRequest.AppChart)
		}

		client, err := cluster.ClientApp()
		if err != nil {
			return apierror.NewInternalError(err)
		}

		// Patch
		patch := fmt.Sprintf(`[{
				"op": "replace",
				"path": "/spec/chartname",
				"value": "%s" }]`,
			updateRequest.AppChart)

		_, err = client.Namespace(app.Meta.Namespace).Patch(ctx, app.Meta.Name, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
		if err != nil {
			return apierror.NewInternalError(err)
		}
	}

	if updateRequest.Instances != nil {
		desired := *updateRequest.Instances

		// Save to configuration
		err := application.ScalingSet(ctx, cluster, app.Meta, desired)
		if err != nil {
			return apierror.NewInternalError(err)
		}
	}

	if len(updateRequest.Environment) > 0 {
		err := application.EnvironmentSet(ctx, cluster, app.Meta, updateRequest.Environment, true)
		if err != nil {
			return apierror.NewInternalError(err)
		}
	}

	if updateRequest.Configurations != nil {
		var okToBind []string

		if len(updateRequest.Configurations) > 0 {
			for _, configurationName := range updateRequest.Configurations {
				_, err := configurations.Lookup(ctx, cluster, namespace, configurationName)
				if err != nil {
					// do not change existing configuration bindings if there is an issue
					if err.Error() == "configuration not found" {
						return apierror.ConfigurationIsNotKnown(configurationName)
					}

					return apierror.NewInternalError(err)
				}

				okToBind = append(okToBind, configurationName)
			}

			err = application.BoundConfigurationsSet(ctx, cluster, app.Meta, okToBind, true)
			if err != nil {
				return apierror.NewInternalError(err)
			}
		} else {
			// remove all bound configurations
			err = application.BoundConfigurationsSet(ctx, cluster, app.Meta, []string{}, true)
			if err != nil {
				return apierror.NewInternalError(err)
			}
		}
	}

	// Only update the app if routes have been set, otherwise just leave it
	// as it is.
	if len(updateRequest.Routes) > 0 {
		client, err := cluster.ClientApp()
		if err != nil {
			return apierror.NewInternalError(err)
		}

		routes := []string{}
		for _, d := range updateRequest.Routes {
			routes = append(routes, fmt.Sprintf("%q", d))
		}

		patch := fmt.Sprintf(`[{
			"op": "replace",
			"path": "/spec/routes",
			"value": [%s] }]`,
			strings.Join(routes, ","))

		_, err = client.Namespace(app.Meta.Namespace).Patch(ctx, app.Meta.Name, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
		if err != nil {
			return apierror.NewInternalError(err)
		}
	}

	// With everything saved, and a workload to update, re-deploy the changed state.
	if app.Workload != nil {
		_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "", nil, nil)
		if apierr != nil {
			return apierr
		}
	}

	response.OK(c)
	return nil
}
