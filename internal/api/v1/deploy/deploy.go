// Package deploy provides the functionality to deploy an application.
package deploy

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
// mutating endpoints, i.e. service and app changes (bindings, environment, scaling).
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
		return nil, apierror.BadRequest(err, "stage id mismatch")
	}

	imageURL := appObj.ImageURL
	instances := *appObj.Configuration.Instances
	environment := appObj.Configuration.Environment
	services := appObj.Configuration.Services
	routes := appObj.Configuration.Routes

	applicationCR, err := application.Get(ctx, cluster, app)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierror.AppIsNotKnown("application resource is missing")
		}
		return nil, apierror.InternalError(err, "failed to get the application resource")
	}

	// owner data to set into new resources
	owner := metav1.OwnerReference{
		APIVersion: applicationCR.GetAPIVersion(),
		Kind:       applicationCR.GetKind(),
		Name:       applicationCR.GetName(),
		UID:        applicationCR.GetUID(),
	}

	deployParams := helm.ChartParameters{
		Cluster:     cluster,
		AppRef:      app,
		Chart:       helm.StandardChart,
		Owner:       owner,
		Environment: environment,
		Services:    services,
		Instances:   instances,
		ImageURL:    imageURL,
		Username:    username,
		StageID:     stageID,
		Routes:      routes,
		Start:       start,
	}

	log.Info("deploying app", "namespace", app.Namespace, "app", app.Name)

	deployParams.ImageURL, err = replaceInternalRegistry(ctx, cluster, imageURL)
	if err != nil {
		return nil, apierror.InternalError(err, "preparing ImageURL registry for use by Kubernetes")
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
// itself and exposed over a NodePort service.
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
	registryDetails, err := registry.GetConnectionDetails(ctx, cluster, helmchart.StagingNamespace, registry.CredentialsSecretName)
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
