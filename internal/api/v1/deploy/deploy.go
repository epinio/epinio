// Package deploy provides the functionality to deploy an application.
package deploy

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/registry"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// DeployApp deploys the referenced application via helm, based on the state held by CRD
// and associated secrets. It is the backend for the API deploypoint, as well as all the
// mutating endpoints, i.e. configuration and app changes (bindings, environment, scaling).
func DeployApp(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef, username, expectedStageID string, origin *models.ApplicationOrigin, start *int64) ([]string, apierror.APIErrors) {
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
		return nil, apierror.NewBadRequest("stage id mismatch", expectedStageID, stageID)
	}

	imageURL := appObj.ImageURL
	routes := appObj.Configuration.Routes
	chartName := appObj.Configuration.AppChart

	deployParams := helm.ChartParameters{
		Context:        ctx,
		Cluster:        cluster,
		AppRef:         app,
		Chart:          chartName,
		Environment:    appObj.Configuration.Environment,
		Configurations: appObj.Configuration.Configurations,
		Instances:      *appObj.Configuration.Instances,
		ImageURL:       imageURL,
		Username:       username,
		StageID:        stageID,
		Routes:         routes,
		Start:          start,
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

	if origin != nil {
		err = application.SetOrigin(ctx, cluster,
			models.NewAppRef(app.Name, app.Namespace), *origin)
		if err != nil {
			return nil, apierror.InternalError(err, "saving the app origin")
		}

		log.Info("saved app origin", "namespace", app.Namespace, "app", app.Name, "origin", *origin)
	}

	return routes, nil
}

// replaceInternalRegistry replaces the registry part of ImageURL with the localhost
// version of the internal Epinio registry if one is found in the registry connection
// details.
// The registry is used by 2 consumers: The staging pod and Kubernetes.
// Staging writes images to it and Kubernetes pulls those images to create the
// application pods.
// A localhost url for the registry only makes sense for Kubernetes because
// for staging it would mean the registry is running inside the staging pod
// (which makes no sense).
// Kubernetes can see a registry on localhost if it is deployed on the cluster
// itself and exposed over a NodePort configuration.
// That's the trick we use, when we deploy the Epinio registry with the
// "force-kube-internal-registry-tls" flag set to "false" in order to allow
// Kubernetes to pull the images without TLS. Otherwise, when the tlsissuer
// that created the registry cert (for the registry Ingress) is not a well
// known one, the user would have to configure Kubernetes to trust that CA.
// This is not a trivial process. For non-production deployments, pulling images
// without TLS is fine.
// When a localhost url doesn't exist, it means one of the following:
// - the Epinio registry is deployed on Kubernetes with a valid cert (e.g. letsencrypt) and the
//   "force-kube-internal-registry-tls" was set to "true" during deployment.
// - the Epinio registry is an external one (if Epinio was deployed that way)
// - a pre-existing image is being deployed (coming from an outer registry, not ours)
func replaceInternalRegistry(ctx context.Context, cluster *kubernetes.Cluster, imageURL string) (string, error) {
	registryDetails, err := registry.GetConnectionDetails(ctx, cluster, helmchart.Namespace(), registry.CredentialsSecretName)
	if err != nil {
		return imageURL, err
	}

	localURL, err := registryDetails.PrivateRegistryURL()
	if err != nil {
		return imageURL, err
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
