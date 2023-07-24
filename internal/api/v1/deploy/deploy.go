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

// Package deploy provides the functionality to deploy an application.
package deploy

import (
	"context"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/registry"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
)

// DeployApp deploys the referenced application via helm, based on the state held by CRD and associated secrets.
// It is the backend for the API deploypoint, as well as all the mutating endpoints,
// i.e. configuration and app changes (bindings, environment, scaling).
func DeployApp(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef, username, expectedStageID string) ([]string, apierror.APIErrors) {
	return deployApp(ctx, cluster, app, username, expectedStageID, false)
}

// DeployAppWithRestart is the same as DeployApp but it will also force Helm to perform a restart of the deployment
func DeployAppWithRestart(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef, username, expectedStageID string) ([]string, apierror.APIErrors) {
	return deployApp(ctx, cluster, app, username, expectedStageID, true)
}

func deployApp(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef, username, expectedStageID string, restart bool) ([]string, apierror.APIErrors) {
	log := requestctx.Logger(ctx)

	appObj, err := application.Lookup(ctx, cluster, app.Namespace, app.Name)
	if err != nil {
		return nil, apierror.InternalError(err)
	}
	if appObj == nil {
		return nil, apierror.AppIsNotKnown(app.Name)
	}

	stageID := appObj.StageID

	if expectedStageID != "" && expectedStageID != stageID {
		return nil, apierror.NewBadRequestError("stage id mismatch").
			WithDetailsf("expectedStageID: [%s] - stageID: [%s]", expectedStageID, stageID)
	}

	imageURL := appObj.ImageURL
	if imageURL == "" {
		return nil, apierror.NewInternalError("cannot deploy app without imageURL")
	}

	// Iterate over the bound configurations to determine their mount path ...

	// (**) See below for explanation
	sort.Strings(appObj.Configuration.Configurations)

	bound := []helm.ConfigParameter{} // Configurations and their mount paths
	service := map[string]int{}       // Seen services, and count of their configurations

	for _, configName := range appObj.Configuration.Configurations {
		config, err := configurations.Lookup(ctx, cluster, app.Namespace, configName)
		if err != nil {
			return nil, apierror.InternalError(err)
		}

		// Default path is config name itself
		path := configName

		// For configurations originating in a service, use the service name instead,
		// possible extended to disambiguate multiple configurations of a single service.
		if config.Origin != "" {
			if serial, ok := service[config.Origin]; !ok {
				path = config.Origin
				service[config.Origin] = 1
			} else {
				// [CS-DISAMBI] With more than one configuration from the same service
				// disambiguate using a serial number
				//
				// Attention! Having sorted the full set of configuration names (see
				// above (**)), the various configurations of the service will
				// always have the same serial (or none, for the first).

				serial = serial + 1
				service[config.Origin] = serial
				path = fmt.Sprintf("%s-%d", config.Origin, serial)
			}
		}

		// Record for passing into the helm core
		bound = append(bound, helm.ConfigParameter{
			Name: configName,
			Path: path,
		})
	}

	routes := appObj.Configuration.Routes
	chartName := appObj.Configuration.AppChart
	domains := domain.MatchMapLoad(ctx, app.Namespace)

	maplog := log.V(1)
	maplog.Info("domain map begin")
	for k, v := range domains {
		maplog.Info("domain map", k, v)
	}
	maplog.Info("domain map end")

	var start *int64
	if restart {
		now := time.Now().UnixNano()
		start = &now
	}

	deployParams := helm.ChartParameters{
		Context:        ctx,
		Cluster:        cluster,
		AppRef:         app,
		Chart:          chartName,
		Environment:    appObj.Configuration.Environment,
		Configurations: bound,
		Instances:      *appObj.Configuration.Instances,
		ImageURL:       imageURL,
		Username:       username,
		StageID:        stageID,
		Routes:         routes,
		Domains:        domains,
		Start:          start,
		Settings:       appObj.Configuration.Settings,
	}

	log.Info("deploying app", "namespace", app.Namespace, "app", app.Name)

	deployParams.ImageURL, err = replaceInternalRegistry(ctx, cluster, imageURL)
	if err != nil {
		return nil, apierror.InternalError(err, "preparing ImageURL registry for use by Kubernetes", imageURL)
	}

	err = helm.Deploy(log, deployParams)
	if err != nil {
		return nil, apierror.InternalError(err)
	}

	// Delete previous staging jobs except for the current one
	if stageID != "" {
		log.Info("app staging drop", "namespace", app.Namespace, "app", app.Name, "stage id", stageID)

		if err := application.Unstage(ctx, cluster, app, stageID); err != nil {
			return nil, apierror.InternalError(err)
		}
	}

	return routes, nil
}

// replaceInternalRegistry replaces the registry part of ImageURL with the localhost
// version of the internal Epinio registry if one is found in the registry connection
// details.
//
// The registry is used by 2 consumers: The staging pod and Kubernetes.  Staging writes
// images to it and Kubernetes pulls those images to create the application pods.
//
// A localhost url for the registry only makes sense for Kubernetes because for staging it
// would mean the registry is running inside the staging pod (which makes no sense).
//
// Kubernetes can see a registry on localhost if it is deployed on the cluster itself and
// exposed over a NodePort configuration.
//
// That's the trick we use, when we deploy the Epinio registry with the
// "force-kube-internal-registry-tls" flag set to "false" in order to allow Kubernetes to
// pull the images without TLS. Otherwise, when the tlsissuer that created the registry
// cert (for the registry Ingress) is not a well known one, the user would have to
// configure Kubernetes to trust that CA.
//
// This is not a trivial process. For non-production deployments, pulling images without
// TLS is fine.
//
// When a localhost url doesn't exist, it means one of the following:
//
// The Epinio registry is deployed on Kubernetes with a valid cert (e.g. letsencrypt) and
// the "force-kube-internal-registry-tls" was set to "true" during deployment.
//
// Or the Epinio registry is an external one (if Epinio was deployed that way)
//
// Or a pre-existing image is being deployed (coming from an outer registry, not ours)

func replaceInternalRegistry(ctx context.Context, cluster *kubernetes.Cluster, imageURL string) (string, error) {
	registryDetails, err := registry.GetConnectionDetails(ctx, cluster, helmchart.Namespace(), registry.CredentialsSecretName)
	if err != nil {
		return "", errors.Wrap(err, "getting connection details")
	}

	localURL, err := registryDetails.PrivateRegistryURL()
	if err != nil {
		return "", errors.Wrap(err, "getting private registry URL")
	}

	if localURL != "" {
		return registryDetails.ReplaceWithInternalRegistry(imageURL)
	}

	return imageURL, nil // no-op
}

func UpdateImageURL(ctx context.Context, cluster *kubernetes.Cluster, app *unstructured.Unstructured, imageURL string) error {
	if err := unstructured.SetNestedField(app.Object, imageURL, "spec", "imageurl"); err != nil {
		return err
	}

	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	namespace, _, err := unstructured.NestedString(app.UnstructuredContent(), "metadata", "namespace")
	if err != nil {
		return err
	}

	_, err = client.Namespace(namespace).Update(ctx, app, metav1.UpdateOptions{})

	return err
}
