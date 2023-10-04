// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"context"
	"net/url"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/routes"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Create handles the API endpoint POST /namespaces/:namespace/applications
// It creates a new and empty application. I.e. without a workload.
func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	username := requestctx.User(ctx).Username

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	var createRequest models.ApplicationCreateRequest
	err = c.BindJSON(&createRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	errorMsgs := validation.IsDNS1123Subdomain(createRequest.Name)
	if len(errorMsgs) > 0 {
		return apierror.NewBadRequestErrorf("Application's name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
	}

	appRef := models.NewAppRef(createRequest.Name, namespace)
	found, err := application.Exists(ctx, cluster, appRef)
	if err != nil {
		return apierror.InternalError(err, "failed to check for app resource")
	}
	if found {
		return apierror.AppAlreadyKnown(createRequest.Name)
	}

	// Sanity check the configurations, if any. IOW anything to be bound
	// has to exist now.  We will check again when the application
	// is deployed, to guard against bound configurations being removed
	// from now till then. While it should not be possible through
	// epinio itself (*), external editing of the relevant
	// resources cannot be excluded from consideration.
	//
	// (*) `epinio configuration delete S` on a bound configuration S will
	//      either reject the operation, or, when forced, unbind S
	//      from the app.

	var theIssues []apierror.APIError

	for _, configurationName := range createRequest.Configuration.Configurations {
		_, err := configurations.Lookup(ctx, cluster, namespace, configurationName)
		if err != nil {
			if err.Error() == "configuration not found" {
				theIssues = append(theIssues, apierror.ConfigurationIsNotKnown(configurationName))
				continue
			}

			theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
			return apierror.NewMultiError(theIssues)
		}
	}

	if len(theIssues) > 0 {
		return apierror.NewMultiError(theIssues)
	}

	var routes []string
	if createRequest.Configuration.Routes != nil {
		// Note: Routes can be empty here!
		for _, d := range createRequest.Configuration.Routes {
			// Strip scheme prefixes, if present
			routeURL, err := url.Parse(d)
			if err != nil {
				return apierror.NewBadRequestError(err.Error()).WithDetails("failed to parse route")
			}
			if routeURL.Scheme != "" {
				d = strings.TrimPrefix(d, routeURL.Scheme+"://")
			}
			routes = append(routes, d)
		}
	} else {
		route, err := domain.AppDefaultRoute(ctx, createRequest.Name, namespace)
		if err != nil {
			return apierror.InternalError(err)
		}
		routes = []string{route}
	}

	apierr := validateRoutes(ctx, cluster, appRef.Name, appRef.Namespace, routes)
	if apierr != nil {
		return apierr
	}

	// Finalize chart selection (system fallback), and verify existence.

	chart := "standard"
	if createRequest.Configuration.AppChart != "" {
		chart = createRequest.Configuration.AppChart
	}

	found, err = appchart.Exists(ctx, cluster, chart)
	if err != nil {
		return apierror.InternalError(err)
	}
	if !found {
		return apierror.AppChartIsNotKnown(chart)
	}

	// Arguments found OK, now we can modify the system state

	err = application.Create(ctx, cluster, appRef, username, routes, chart,
		createRequest.Configuration.Settings)
	if err != nil {
		return apierror.InternalError(err)
	}

	desired := DefaultInstances
	if createRequest.Configuration.Instances != nil {
		desired = *createRequest.Configuration.Instances
	}

	err = application.ScalingSet(ctx, cluster, appRef, desired)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Save configuration information.
	err = application.BoundConfigurationsSet(ctx, cluster, appRef,
		createRequest.Configuration.Configurations, true)
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

func validateRoutes(ctx context.Context, cluster *kubernetes.Cluster, appName, namespace string, desiredRoutes []string) apierror.APIErrors {
	desiredRoutesMap := map[string]struct{}{}
	for _, desiredRoute := range desiredRoutes {
		desiredRoutesMap[desiredRoute] = struct{}{}
	}

	ingressList, err := cluster.Kubectl.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return apierror.InternalError(err)
	}

	issues := []apierror.APIError{}

	for _, ingress := range ingressList.Items {
		ingressIssues := validateIngress(desiredRoutesMap, appName, namespace, ingress)
		if len(ingressIssues) > 0 {
			issues = append(issues, ingressIssues...)
		}
	}

	if len(issues) > 0 {
		return apierror.NewMultiError(issues)
	}
	return nil
}

// validateIngress checks if the desiredRoutesMap is in conflict with the passed
// ingress object. Conflict means, the ingress already defines one of the desired
// routes and it belongs to another or an unknown app.
func validateIngress(desiredRoutesMap map[string]struct{}, appName, namespace string, ingress networkingv1.Ingress) []apierror.APIError {
	issues := []apierror.APIError{}

	routes, err := routes.FromIngress(ingress)
	if err != nil {
		return append(issues, apierror.InternalError(err))
	}

	for _, route := range routes {
		routeStr := route.String()

		// if a desired route is present within the ingresses then we have to check
		// if it is already owned by the same app
		if _, found := desiredRoutesMap[routeStr]; found {
			ingressAppName, found := ingress.GetLabels()["app.kubernetes.io/name"]
			if !found {
				err := apierror.NewBadRequestErrorf("route is already owned by an unknown app").
					WithDetailsf("app: [%s], namespace: [%s], ingress: [%s]", appName, namespace, ingress.Name)
				issues = append(issues, err)
				continue
			}

			// the ingress route is owned by another app
			if appName != ingressAppName || namespace != ingress.Namespace {
				err := apierror.NewBadRequestErrorf("route '%s' already exists", route).
					WithDetailsf("route is already owned by app [%s] in namespace [%s]", ingressAppName, ingress.Namespace)
				issues = append(issues, err)
			}
		}
	}

	return issues
}
