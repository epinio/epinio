package service

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// ServiceApps handles the API endpoint GET /namespaces/:org/serviceapps
// It returns a map from services to the apps they are bound to, in
// the specified org.  Internally it asks each app in the org for its
// bound services and then inverts that map to get the desired result.
func (hc Controller) ServiceApps(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	org := c.Param("org")

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

	appsOf, err := servicesToApps(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, appsOf)
	return nil
}

// serviceKey constructs a single key string from service and namespace names, for the
// `servicesToApps` map, when used for services and apps across all namespaces. It uses
// ASCII NUL (\000) as the separator character. NUL is forbidden to occur in the names
// themselves. This should make it impossible to construct two different pairs of
// service/namespace names which map to the same key.
func serviceKey(name, namespace string) string {
	return fmt.Sprintf("%s\000%s", name, namespace)
}

// servicesToApps is a helper to Index and Delete. It produces a map from service
// instances names to application names, the apps bound to each service.
func servicesToApps(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (map[string]models.AppList, error) {
	// Determine apps bound to services
	// (inversion of services bound to apps)
	// Literally query apps in the namespace for their services and invert.

	var appsOf = map[string]models.AppList{}

	apps, err := application.List(ctx, cluster, namespace)
	if err != nil {
		return nil, err
	}

	if namespace == "" {
		// services are collected across all namespaces.
		// Key the map by service and namespace!
		// Because services of the same name can exist in
		// different namespaces, and different binding states.

		for _, app := range apps {
			for _, bound := range app.Configuration.Services {
				key := serviceKey(bound, app.Meta.Org)
				if theapps, found := appsOf[key]; found {
					appsOf[key] = append(theapps, app)
				} else {
					appsOf[key] = models.AppList{app}
				}
			}
		}
	} else {
		for _, app := range apps {
			for _, bound := range app.Configuration.Services {
				if theapps, found := appsOf[bound]; found {
					appsOf[bound] = append(theapps, app)
				} else {
					appsOf[bound] = models.AppList{app}
				}
			}
		}
	}

	return appsOf, nil
}
