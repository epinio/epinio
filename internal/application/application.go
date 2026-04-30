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

// Package application collects the structures and functions that deal with application
// workloads on k8s
package application

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"

	epinioappv1 "github.com/epinio/application/api/v1"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	apibatchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

const EpinioApplicationAreaLabel = "epinio.io/area"

type JobLister interface {
	ListJobs(
		ctx context.Context,
		namespace, selector string,
	) (*apibatchv1.JobList, error)
}

// ValidateCV checks the custom values against the declarations.
// It reports as many issues as it can find.
func ValidateCV(
	cv models.ChartValueSettings,
	decl map[string]models.ChartSetting,
) []error {
	// See also internal/helm Deploy(). A last-minute check to catch any changes
	// possibly landing in the time window between the check here and the actual
	// deployment.

	var issues []error

	/* Pattern to strip array index syntax from the actual key to determine the
		 underlying base setting to check against. Note that this handles inner
		 array syntax too.

	Examples:                                   KEY                           KEYBASE
	  --set 'keycloak.ingress.hosts[0]=auth1' ~ 'keycloak.ingress.hosts[0]' ~ 'keycloak.ingress.hosts'
		--set 'servers[0].port=80'              ~ 'servers[0].port'           ~ 'servers.port'
	*/

	rex := regexp.MustCompile(`\[[^]]\]`)

	for key, value := range cv {
		keybase := rex.ReplaceAllString(key, "")

		spec, found := decl[keybase]
		if !found {
			// Shorten the key incrementally to see if a prefix exists and is a map.

			nestedmap := false
			pieces := strings.Split(keybase, ".")
			pieces = pieces[0 : len(pieces)-1]

			for len(pieces) > 0 {
				prefix := strings.Join(pieces, ".")

				spec, found := decl[prefix]
				if found && spec.Type == "map" {
					nestedmap = true
					break
				}

				pieces = pieces[0 : len(pieces)-1]
			}

			if !nestedmap {
				issues = append(issues, fmt.Errorf(`setting "%s": Not known`, keybase))
			}
			continue
		}

		// Maps are not checked deeper.
		if spec.Type == "map" {
			continue
		}

		// Note: The interface{} result for the properly typed value is ignored here.
		// We do not care about the value, just that it is ok.

		_, err := helm.ValidateField(keybase, value, spec)
		if err != nil {
			issues = append(issues, err)
		}
	}
	return issues
}

// Create generates a new kube app resource in the namespace of the namespace.
// Note that this is the passive resource holding the app's configuration.
// It is not the active workload.
func Create(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	username string,
	routes []string,
	chart string,
	settings models.ChartValueSettings,
) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	// we create the appCRD in the namespace
	obj := &epinioappv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				models.EpinioCreatedByAnnotation: username,
			},
		},
		Spec: epinioappv1.AppSpec{
			Routes:    routes,
			Origin:    epinioappv1.AppOrigin{},
			ChartName: chart,
			Settings:  settings,
		},
	}

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	us := &unstructured.Unstructured{Object: u}
	us.SetAPIVersion("application.epinio.io/v1")
	us.SetKind("App")
	us.SetName(app.Name)

	// [NO-ROUTES] Note: An empty routes slice is not stored in the app kube resource.
	// (`omitempty`!) See `DesiredRoutes` for the converse. Treat missing field as empty
	// slice. Same marker as here.

	_, err = client.Namespace(app.Namespace).Create(ctx, us, metav1.CreateOptions{})
	return err
}

// Get returns the application resource from the cluster.  This should be
// changed to return a typed application struct, like epinioappv1.App if needed
// in the future.
func Get(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
) (*unstructured.Unstructured, error) {
	client, err := cluster.ClientApp()
	if err != nil {
		return nil, err
	}

	return client.Namespace(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
}

// Exists checks if the named application exists or not, and returns an
// appropriate boolean flag
func Exists(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
) (bool, error) {
	_, err := Get(ctx, cluster, app)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// IsCurrentlyStaging returns true if the named application is staging
// (there is an active Job for this application). If this information is needed
// for more than one application use StagingStatuses instead.
func IsCurrentlyStaging(
	ctx context.Context,
	cluster JobLister,
	namespace,
	appName string,
) (bool, error) {
	staging, err := stagingStatus(ctx, cluster, namespace, appName)
	if err != nil {
		return false, err
	}
	status := staging[EncodeConfigurationKey(appName, namespace)]
	return status == models.ApplicationStagingActive, nil
}

// StagingStatuses returns a map of applications and their staging statuses
func StagingStatuses(
	ctx context.Context,
	cluster JobLister,
	namespace string,
) (map[ConfigurationKey]models.ApplicationStagingStatus, error) {
	return stagingStatus(ctx, cluster, namespace, "")
}

/*
stagingStatus is a utility function loading a map of the status of the
application's staging jobs (active, done, error).  If no appName is specified
it will load a complete map, otherwise the map will contain only the status
of the job of the specified app
*/
func stagingStatus(
	ctx context.Context,
	cluster JobLister,
	namespace,
	appName string,
) (map[ConfigurationKey]models.ApplicationStagingStatus, error) {
	stagingJobsMap := make(map[ConfigurationKey]models.ApplicationStagingStatus)

	// filter the jobs in the namespace
	labelsMap := make(map[string]string)

	if namespace != "" {
		labelsMap["app.kubernetes.io/part-of"] = namespace
	}

	if appName != "" {
		labelsMap["app.kubernetes.io/name"] = appName
	}

	selector := labels.Set(labelsMap).AsSelector().String()
	jobList, err := cluster.ListJobs(ctx, helmchart.Namespace(), selector)
	if err != nil {
		return nil, err
	}

	completed := func(condition apibatchv1.JobCondition) bool {
		return condition.Status == v1.ConditionTrue && condition.Type == apibatchv1.JobComplete
	}

	failed := func(condition apibatchv1.JobCondition) bool {
		return condition.Status == v1.ConditionTrue && condition.Type == apibatchv1.JobFailed
	}

	jobStaging := func(job apibatchv1.Job) models.ApplicationStagingStatus {
		for _, condition := range job.Status.Conditions {
			if failed(condition) {
				return models.ApplicationStagingFailed
			}
			if completed(condition) {
				// Terminal, not staging
				return models.ApplicationStagingDone
			}
		}
		// No terminal condition found on the job, it is actively staging
		return models.ApplicationStagingActive
	}

	for _, job := range jobList.Items {
		appName := job.GetLabels()["app.kubernetes.io/name"]
		namespace := job.GetLabels()["app.kubernetes.io/part-of"]
		stagingJobsMap[EncodeConfigurationKey(appName, namespace)] = jobStaging(job)
	}

	return stagingJobsMap, nil
}

func updateAppDataMapWithStagingJobStatus(
	appDataMap map[ConfigurationKey]AppData,
	stagingJobsMap map[ConfigurationKey]models.ApplicationStagingStatus,
) map[ConfigurationKey]AppData {
	for appName, stagingStatus := range stagingJobsMap {
		appData := appDataMap[appName]
		appData.staging = stagingStatus
		appDataMap[appName] = appData
	}
	return appDataMap
}

// Lookup locates the named application (and namespace).
func Lookup(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace,
	appName string,
) (*models.App, error) {
	meta := models.NewAppRef(appName, namespace)

	ok, err := Exists(ctx, cluster, meta)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	app := meta.App()

	err = fetch(ctx, cluster, app)
	return app, err
}

/*
ListAppRefs returns an app reference for every application resource in the
specified namespace. If no namespace is specified (empty string) then apps
across all namespaces are returned.
*/
func ListAppRefs(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace string,
) ([]models.AppRef, error) {
	client, err := cluster.ClientApp()
	if err != nil {
		return nil, err
	}

	list, err := client.Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	apps := make([]models.AppRef, 0, len(list.Items))
	for _, app := range list.Items {
		// XXX created-at!
		apps = append(apps, models.NewAppRef(app.GetName(), app.GetNamespace()))
	}

	return apps, nil
}

type AppData struct {
	scaling  *v1.Secret
	bound    *v1.Secret
	env      *v1.Secret
	services *v1.Secret
	routes   []string
	pods     []v1.Pod
	staging  models.ApplicationStagingStatus
}

/*
List returns a list of all available apps in the specified namespace.
If no namespace	is specified (empty string) then apps across all namespaces
are returned.
*/
func List(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace string,
) (models.AppList, error) {

	// Verify namespace, if specified
	// This is actually handled by `NamespaceMiddleware`.

	// Fast batch queries to load all relevant resources in as few kube calls as
	// possible.

	// I. Get the application resources for all apps, deployed or not

	client, err := cluster.ClientApp()
	if err != nil {
		return nil, err
	}
	appCRList, err := client.Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// II. Load the auxiliary application data found in adjacent kube Secret
	// resources (environment, scaling, bound configs).

	secrets, err := cluster.Kubectl.CoreV1().Secrets(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=epinio",
		},
	)
	if err != nil {
		return nil, err
	}

	appAuxiliary := makeAuxiliaryMap(secrets.Items)

	// III. The pods for the deployed apps.

	appAuxiliary, err = AddApplicationPods(appAuxiliary, ctx, cluster, namespace)
	if err != nil {
		return nil, err
	}

	// IV. Actual application routes from the ingresses

	appAuxiliary, err = AddActualApplicationRoutes(
		appAuxiliary,
		ctx,
		cluster,
		namespace,
	)
	if err != nil {
		return nil, err
	}

	// V. Pod metrics and replica information

	metrics, err := GetPodMetrics(ctx, cluster, namespace)
	if err != nil {
		// While the error is ignored, as the server can operate without metrics, and while
		// the missing metrics will be noted in the data shown to the user, it is logged so
		// that the operator can see this as well.
		helpers.Logger.Errorw("metrics not available", "error", err)
	}

	// VI. load the statuses of all staging jobs

	stagingStatuses, err := StagingStatuses(ctx, cluster, namespace)
	if err != nil {
		return nil, err
	}
	appAuxiliary = updateAppDataMapWithStagingJobStatus(
		appAuxiliary,
		stagingStatuses,
	)

	// Fuse the loaded resources into full application structures.

	result := models.AppList{}

	for _, appCR := range appCRList.Items {
		app, err := aggregate(ctx, cluster, appCR, appAuxiliary, metrics)
		if err != nil {
			return result, err
		}
		if app != nil {
			result = append(result, *app)
		}
	}

	return result, nil
}

/*
Delete removes the named application, its workload (if active), bindings
(if any), the stored application sources, and any staging jobs from when the
application was staged (if active). Waits for the application's deployment's
pods to disappear (if active).
*/
func Delete(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	deleteImage bool,
) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	log := helpers.Logger.With("component", "ApplicationDelete")

	// Get image URL before deleting the app resource (needed for image deletion)
	var imageURL string
	if deleteImage {
		appCR, err := Get(ctx, cluster, appRef)
		if err != nil {
			log.Errorw("Failed to get application to retrieve image URL, skipping image deletion", "error", err, "app", appRef.Name)
		} else {
			imageURL, err = ImageURL(appCR)
			if err != nil {
				log.Errorw("Failed to get image URL from application, skipping image deletion", "error", err, "app", appRef.Name)
			} else if imageURL == "" {
				log.Infow("No image URL found in application, skipping image deletion", "app", appRef.Name)
			}
		}
	}

	// Ignore `not found` errors - App exists, without workload.
	err = helm.Remove(cluster, appRef)
	if err != nil && !strings.Contains(err.Error(), "release: not found") {
		return err
	}

	// Keep existing code to remove the CRD and everything it owns. Only the
	// workload resources needed their own removal to ensure that helm information
	// stays consistent.

	// delete application resource, will cascade and delete dependents like
	// environment variables, bindings, etc.
	err = client.Namespace(appRef.Namespace).Delete(
		ctx,
		appRef.Name,
		metav1.DeleteOptions{},
	)
	if err != nil {
		return err
	}

	// Delete container image from registry if requested
	if deleteImage && imageURL != "" {
		err = deleteContainerImage(ctx, cluster, imageURL)
		if err != nil {
			// Log the error but don't fail the deletion - the app is already deleted
			log.Errorw("Failed to delete container image from registry", "error", err, "image", imageURL)
		}
	}

	// delete old staging resources in namespace (helmchart.Namespace())
	err = Unstage(ctx, cluster, appRef, "")
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// delete staging PVC (the one that holds the "source" and "cache" workspaces)
	err = deleteCacheStagePVC(ctx, cluster, appRef)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	err = deleteSourceBlobsStagePVC(ctx, cluster, appRef)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	err = cluster.WaitForPodBySelectorMissing(ctx,
		appRef.Namespace,
		fmt.Sprintf("app.kubernetes.io/name=%s", appRef.Name),
		duration.ToDeployment())
	if err != nil {
		return err
	}

	return nil
}

// deleteContainerImage deletes the container image from the registry
func deleteContainerImage(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	imageURL string,
) error {
	// Get registry connection details
	connectionDetails, err := registry.GetConnectionDetails(
		ctx,
		cluster,
		helmchart.Namespace(),
		registry.CredentialsSecretName,
	)
	if err != nil {
		return errors.Wrap(err, "getting registry connection details")
	}

	if len(connectionDetails.RegistryCredentials) == 0 {
		return errors.New("no registry credentials found")
	}

	// Use the first set of credentials (typically there's only one)
	credentials := connectionDetails.RegistryCredentials[0]

	// Extract registry URL from image URL to match credentials
	imageRegistryURL, _, err := registry.ExtractImageParts(imageURL)
	if err != nil {
		return errors.Wrap(err, "extracting registry URL from image")
	}

	// Find matching credentials for the image's registry
	var matchingCreds registry.RegistryCredentials
	found := false
	for _, cred := range connectionDetails.RegistryCredentials {
		// Check if the registry URL matches (with or without namespace suffix)
		if cred.URL == imageRegistryURL || strings.HasPrefix(imageRegistryURL, cred.URL) {
			matchingCreds = cred
			found = true
			break
		}
	}

	if !found {
		// If no exact match, use the first credentials (might work for some registries)
		matchingCreds = credentials
		helpers.Logger.Infow(
			"No exact registry match found, using first available credentials",
			"imageRegistry",
			imageRegistryURL,
		)
	}

	// Get TLS config to handle self-signed certificates
	var tlsConfig *tls.Config
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		rootCAs = x509.NewCertPool()
	}
	certsAdded := false

	// Check for registry certificate secret (for self-signed registry certificates)
	// This is the primary way to trust self-signed registry certificates
	registryCertSecret := os.Getenv("REGISTRY_CERTIFICATE_SECRET")
	if registryCertSecret != "" {
		secret, err := cluster.GetSecret(ctx, helmchart.Namespace(), registryCertSecret)
		if err == nil {
			if certData, found := secret.Data["tls.crt"]; found {
				if rootCAs.AppendCertsFromPEM(certData) {
					certsAdded = true
					helpers.Logger.Infow(
						"Added registry certificate from secret to TLS config",
						"secret",
						registryCertSecret,
					)
				}
			}
		} else {
			helpers.Logger.Infow(
				"Registry certificate secret not found, continuing without it",
				"secret",
				registryCertSecret,
				"error",
				err,
			)
		}
	}

	// Also add cluster's CA data if available (for cluster-internal registries)
	if cluster.RestConfig != nil {
		tlsClientConfig := cluster.RestConfig.TLSClientConfig
		if tlsClientConfig.Insecure {
			// If cluster config says insecure, skip verification
			tlsConfig = &tls.Config{
				InsecureSkipVerify: true, // nolint:gosec // Controlled by cluster config
				MinVersion:         tls.VersionTLS12,
			}
		} else if tlsClientConfig.CAData != nil {
			// Add cluster CA to the cert pool
			if rootCAs.AppendCertsFromPEM(tlsClientConfig.CAData) {
				certsAdded = true
			}
		}
	}

	// Build TLS config based on what we have
	if tlsConfig == nil {
		// Check if this is an internal registry (cluster.local domain)
		isInternalRegistry := strings.Contains(imageRegistryURL, ".svc.cluster.local") ||
			strings.Contains(imageRegistryURL, "127.0.0.1") ||
			strings.Contains(imageRegistryURL, "localhost")

		if certsAdded {
			// We have certificates in the pool, use them
			tlsConfig = &tls.Config{
				RootCAs:    rootCAs,
				MinVersion: tls.VersionTLS12,
			}
		} else if isInternalRegistry {
			// Internal registry with no certificate found - skip verification
			// This is common for internal registries with self-signed certs
			tlsConfig = &tls.Config{
				InsecureSkipVerify: true, // nolint:gosec // Internal registry with self-signed cert
				MinVersion:         tls.VersionTLS12,
			}
			helpers.Logger.Infow(
				"No registry certificate found, skipping TLS verification for internal registry",
				"registry",
				imageRegistryURL,
			)
		} else {
			// External registry - use system cert pool (may fail if cert is not trusted)
			tlsConfig = &tls.Config{
				RootCAs:    rootCAs,
				MinVersion: tls.VersionTLS12,
			}
		}
	}

	// Delete the image
	return registry.DeleteImage(ctx, imageURL, matchingCreds, tlsConfig)
}

// deleteCacheStagePVC removes the kube PVC resource which was used to hold the application
// sources for staging.
func deleteCacheStagePVC(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
) error {
	return cluster.Kubectl.CoreV1().PersistentVolumeClaims(
		helmchart.Namespace(),
	).Delete(ctx, appRef.MakeCachePVCName(), metav1.DeleteOptions{})
}

func deleteSourceBlobsStagePVC(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
) error {
	return cluster.Kubectl.CoreV1().
		PersistentVolumeClaims(
			helmchart.Namespace(),
		).Delete(ctx, appRef.MakeSourceBlobsPVCName(), metav1.DeleteOptions{})
}

/*
AppChart returns the app chart (to be) used for application deployment, if
one exists. It returns an empty string otherwise. The information is pulled
out of the app resource itself, saved there by the deploy endpoint.
*/
func AppChart(app *unstructured.Unstructured) (string, error) {
	chartName, _, err := unstructured.NestedString(
		app.UnstructuredContent(),
		"spec",
		"chartname",
	)
	if err != nil {
		return "", errors.New("chartname should be string")
	}

	return chartName, nil
}

/*
Settings returns the app chart customization settings used for application
deployment. It returns an empty slice otherwise. The information is pulled
out of the app resource itself, saved there by the deploy endpoint.
*/
func Settings(app *unstructured.Unstructured) (models.ChartValueSettings, error) {
	settings, _, err := unstructured.NestedStringMap(
		app.UnstructuredContent(),
		"spec",
		"settings",
	)
	if err != nil {
		return models.ChartValueSettings{}, errors.New("chartname should be string")
	}

	return settings, nil
}

/*
StageID returns the stage ID of the last attempt at staging, if one exists.
It returns an empty string otherwise. The information is pulled out of the
app resource itself, saved there by the staging endpoint. Note that
success/failure of staging is immaterial to this.
*/
func StageID(app *unstructured.Unstructured) (string, error) {
	stageID, _, err := unstructured.NestedString(
		app.UnstructuredContent(),
		"spec",
		"stageid",
	)
	if err != nil {
		return "", errors.New("stageid should be string")
	}

	return stageID, nil
}

/*
ImageURL returns the image url of the currently running build, if one exists.
It returns an empty string otherwise. The information is pulled out of the
app resource itself, saved there by the deploy endpoint.
*/
func ImageURL(app *unstructured.Unstructured) (string, error) {
	imageURL, _, err := unstructured.NestedString(
		app.UnstructuredContent(),
		"spec",
		"imageurl",
	)
	if err != nil {
		return "", errors.New("imageurl should be string")
	}

	return imageURL, nil
}

/*
BuilderURL returns the builder url of the currently running build, if one
exists. It returns an empty string otherwise. The information is pulled out
of the app resource	itself, saved there by the deploy endpoint.
*/
func BuilderURL(app *unstructured.Unstructured) (string, error) {
	builderURL, _, err := unstructured.NestedString(
		app.UnstructuredContent(),
		"spec",
		"builderimage",
	)
	if err != nil {
		return "", errors.New("builderimage should be string")
	}

	return builderURL, nil
}

/*
Unstage removes staging resources. It deletes either all Jobs of the named
application, or all but stageIDCurrent. It also deletes the staged objects
from the S3 storage	except for the current one.
*/
func Unstage(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	stageIDCurrent string,
) error {
	s3ConnectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster,
		helmchart.Namespace(), helmchart.S3ConnectionDetailsSecretName)
	if err != nil {
		return errors.Wrap(
			err,
			"fetching the S3 connection details from the Kubernetes secret",
		)
	}
	s3m, err := s3manager.New(s3ConnectionDetails)
	if err != nil {
		return errors.Wrap(err, "creating an S3 manager")
	}

	jobs, err := cluster.ListJobs(ctx, helmchart.Namespace(),
		fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s",
			appRef.Name, appRef.Namespace))

	if err != nil {
		return err
	}

	var currentJob *apibatchv1.Job
	for i, job := range jobs.Items {
		id := job.Labels[models.EpinioStageIDLabel]
		// stageIDCurrent is either empty or the id to keep
		if stageIDCurrent != "" && stageIDCurrent == id {
			currentJob = &jobs.Items[i]
			continue
		}

		err := cluster.DeleteJob(ctx, job.Namespace, job.Name)
		if err != nil {
			return err
		}

		// And the associated secret holding the job environment
		err = cluster.DeleteSecret(ctx, job.Namespace, job.Name)
		if err != nil {
			return err
		}
	}

	// Cleanup s3 objects
	for _, job := range jobs.Items {
		// skip prs with the same blob as the current one (including the current one)
		if currentJob != nil && job.Labels[models.EpinioStageBlobUIDLabel] == currentJob.Labels[models.EpinioStageBlobUIDLabel] {
			continue
		}

		if err = s3m.DeleteObject(ctx, job.Labels[models.EpinioStageBlobUIDLabel]); err != nil {
			return err
		}
	}

	return nil
}

/*
Logs method writes log lines to the specified logChan. The caller can stop
the logging with the ctx cancelFunc. It's also the callers responsibility to
close the logChan when done. When stageID is an empty string, no staging logs
are returned. If it is set, LogParameters represents the log filtering
parameters.
*/
type LogParameters struct {
	Tail              *int64
	Since             *time.Duration
	SinceTime         *time.Time
	Follow            bool
	IncludeContainers []string // List of container names/patterns to include (regex patterns)
	ExcludeContainers []string // List of container names/patterns to exclude (regex patterns)
}

// buildContainerIncludePattern builds the regex pattern for including containers.
// Returns the pattern and whether the user specified an include filter.
func buildContainerIncludePattern(logParams *LogParameters) (string, bool, error) {
	containerQueryPattern := ".*"
	hasUserIncludeFilter := false

	if logParams == nil || len(logParams.IncludeContainers) == 0 {
		return containerQueryPattern, hasUserIncludeFilter, nil
	}

	// Filter out any empty strings that might have slipped through
	validIncludes := make([]string, 0, len(logParams.IncludeContainers))
	for _, container := range logParams.IncludeContainers {
		if trimmed := strings.TrimSpace(container); trimmed != "" {
			validIncludes = append(validIncludes, trimmed)
		}
	}

	if len(validIncludes) == 0 {
		return "", false, fmt.Errorf("include_containers parameter contains no valid container names")
	}

	hasUserIncludeFilter = true

	// Escape special regex characters and join with |
	escapedIncludes := make([]string, len(validIncludes))
	for i, container := range validIncludes {
		// If the pattern already looks like a regex (contains special chars), use as-is
		// Otherwise, treat as literal container name
		if strings.ContainsAny(container, ".*+?^$|[]{}()\\") {
			escapedIncludes[i] = container
		} else {
			// Escape as literal container name
			escapedIncludes[i] = regexp.QuoteMeta(container)
		}
	}
	containerQueryPattern = strings.Join(escapedIncludes, "|")

	return containerQueryPattern, hasUserIncludeFilter, nil
}

// buildContainerExcludePattern builds the regex pattern for excluding containers.
func buildContainerExcludePattern(logParams *LogParameters, hasUserIncludeFilter bool) (*regexp.Regexp, error) {
	var excludeContainerPatterns []string

	// Only apply default linkerd exclusion if user hasn't specified include_containers
	// This allows users to explicitly include linkerd containers when needed
	if !hasUserIncludeFilter {
		excludeContainerPatterns = []string{"linkerd-(proxy|init)"}
	}

	// Add user-specified exclusions
	if logParams != nil && len(logParams.ExcludeContainers) > 0 {
		// Initialize if not already set (shouldn't happen, but be safe)
		if excludeContainerPatterns == nil {
			excludeContainerPatterns = []string{}
		}
		for _, container := range logParams.ExcludeContainers {
			// Filter out empty strings
			trimmed := strings.TrimSpace(container)
			if trimmed == "" {
				continue
			}
			// If the pattern already looks like a regex, use as-is
			// Otherwise, treat as literal container name
			if strings.ContainsAny(trimmed, ".*+?^$|[]{}()\\") {
				excludeContainerPatterns = append(excludeContainerPatterns, trimmed)
			} else {
				// Escape as literal container name
				excludeContainerPatterns = append(excludeContainerPatterns, regexp.QuoteMeta(trimmed))
			}
		}
	}

	// Build the final exclude pattern by combining all exclusions
	if len(excludeContainerPatterns) > 0 {
		// Combine all exclusion patterns with |
		combinedExcludePattern := strings.Join(excludeContainerPatterns, "|")
		excludeContainerQuery, err := regexp.Compile(combinedExcludePattern)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude_containers pattern: %w", err)
		}
		return excludeContainerQuery, nil
	}

	// No exclusions (shouldn't happen with default, but handle gracefully)
	return nil, nil
}

// applyLogParameters applies log filtering parameters to the tailer config.
func applyLogParameters(config *tailer.Config, logParams *LogParameters) {
	if logParams == nil {
		return
	}

	helpers.Logger.Info("applying log parameters", "params", logParams)

	// Handle line limiting
	if logParams.Tail != nil {
		config.TailLines = logParams.Tail
		helpers.Logger.Info("applied tail parameter", "tail", *logParams.Tail)
	}

	// Handle time-based filtering
	if logParams.SinceTime != nil {
		config.SinceTime = logParams.SinceTime
		helpers.Logger.Info(
			"applying since time parameter | ",
			"since_time: ",
			*logParams.SinceTime,
		)
	} else if logParams.Since != nil {
		config.Since = *logParams.Since
		helpers.Logger.Info(
			"applied since parameter | ",
			"since: ",
			*logParams.Since,
		)
	}
}

// then only logs from that staging process are returned.
func Logs(
	ctx context.Context,
	logChan chan tailer.ContainerLogLine,
	wg *sync.WaitGroup,
	cluster *kubernetes.Cluster,
	app,
	stageID,
	namespace string,
	logParams *LogParameters,
) error {
	selector := labels.NewSelector()

	var selectors [][]string
	if stageID == "" {
		selectors = [][]string{
			{"app.kubernetes.io/component", "application"},
			{"app.kubernetes.io/part-of", namespace},
			{"app.kubernetes.io/name", app},
		}
	} else {
		selectors = [][]string{
			{"app.kubernetes.io/component", "staging"},
			{models.EpinioStageIDLabel, stageID},
			{"app.kubernetes.io/part-of", namespace},
		}
	}

	for _, selectorPair := range selectors {
		req, err := labels.NewRequirement(
			selectorPair[0],
			selection.Equals,
			[]string{selectorPair[1]},
		)
		if err != nil {
			return err
		}
		selector = selector.Add(*req)
	}

	// Build container filtering regex patterns
	containerQueryPattern, hasUserIncludeFilter, err := buildContainerIncludePattern(logParams)
	if err != nil {
		return err
	}

	excludeContainerQuery, err := buildContainerExcludePattern(logParams, hasUserIncludeFilter)
	if err != nil {
		return err
	}

	// Compile the include pattern with error handling
	containerQueryRegex, err := regexp.Compile(containerQueryPattern)
	if err != nil {
		return fmt.Errorf("invalid include_containers pattern: %w", err)
	}

	config := &tailer.Config{
		ContainerQuery:        containerQueryRegex,
		ExcludeContainerQuery: excludeContainerQuery,
		Exclude:               nil,
		Include:               nil,
		Timestamps:            true,
		SinceTime:             nil,
		Since:                 0,
		AllNamespaces:         true,
		LabelSelector:         selector,
		TailLines:             getTailLines(),
		Namespace:             "",
		PodQuery:              regexp.MustCompile(".*"),
	}

	if stageID != "" {
		config.Ordered = true
	}

	// Apply log parameters if provided
	applyLogParameters(config, logParams)
	if logParams != nil {
		helpers.Logger.Infow("applying log parameters", "params", logParams)

		// Handle line limiting
		if logParams.Tail != nil {
			config.TailLines = logParams.Tail
			helpers.Logger.Infow("applied tail parameter", "tail", *logParams.Tail)
		}

		// Handle time-based filtering
		if logParams.SinceTime != nil {
			config.SinceTime = logParams.SinceTime
			helpers.Logger.Infow("applying since time parameter",
				"since_time", *logParams.SinceTime,
			)
		} else if logParams.Since != nil {
			config.Since = *logParams.Since
			helpers.Logger.Infow("applied since parameter",
				"since", *logParams.Since,
			)
		}
	}

	// Use follow from logParams if provided, otherwise default to false
	follow := false
	if logParams != nil {
		follow = logParams.Follow
	}

	if follow {
		helpers.Logger.Infow("stream")
		return tailer.StreamLogs(ctx, logChan, wg, config, cluster)
	}

	helpers.Logger.Infow("fetch")
	return tailer.FetchLogs(ctx, logChan, wg, config, cluster)
}

// makeAuxiliaryMap restructures the data from the auxiliary secrets into a map
// for quick access during the following data fusion
func makeAuxiliaryMap(secrets []v1.Secret) map[ConfigurationKey]AppData {
	/*
		Note: The returned secrets are a mix of scaling instructions, bound
		configurations, and environment assignments. Split them into separate maps
		as per their area (*). Key the maps by namespace and name of their
		controlling application for quick access in the	aggregation step.

		(*) Label "epinio.io/area": "environment"|"scaling"|"configuration"
	*/

	result := map[ConfigurationKey]AppData{}

	for _, s := range secrets {
		area, found := s.Labels["epinio.io/area"]
		if !found {
			continue
		}
		app, found := s.Labels["app.kubernetes.io/name"]
		if !found {
			continue
		}

		key := EncodeConfigurationKey(app, s.Namespace)

		if _, found := result[key]; !found {
			result[key] = AppData{}
		}

		data := result[key]
		secretToAssign := s // avoid loop alias warning

		switch area {
		case "scaling":
			data.scaling = &secretToAssign
		case "configuration":
			data.bound = &secretToAssign
		case "environment":
			data.env = &secretToAssign
		case "service":
			data.services = &secretToAssign
		default:
			// ignore secret
		}

		result[key] = data
	}

	return result
}

// Aggregate is an internal helper for List. It merges the information from an
// application resource and adjacent secrets, pods, metrics, etc. into a proper
// application structure.
func aggregate(ctx context.Context,
	cluster *kubernetes.Cluster,
	appCR unstructured.Unstructured,
	auxiliary map[ConfigurationKey]AppData,
	metrics map[string]metricsv1beta1.PodMetrics,
) (*models.App, error) {
	appName := appCR.GetName()
	namespace := appCR.GetNamespace()

	key := EncodeConfigurationKey(appName, namespace)

	// I. Unpack the auxiliary data in the various secrets
	//    Note: missing aux data, all or parts indicates an app in deletion and
	//		not fully gone.
	//    We signal them as not existing, instead of erroring out

	aux, found := auxiliary[key]
	if !found {
		return nil, nil
	}
	if aux.env == nil {
		return nil, nil
	}
	if aux.bound == nil {
		return nil, nil
	}
	if aux.scaling == nil {
		return nil, nil
	}

	instances, err := ScalingFromSecret(aux.scaling)
	if err != nil {
		// parse errors only, i.e. bad data.
		return nil, errors.Wrap(err, "finding scaling")
	}

	configurations := BoundConfigurationNamesFromSecret(aux.bound)
	environment := EnvironmentFromSecret(aux.env)
	appPods := aux.pods
	appRoutes := aux.routes
	var services []string
	if aux.services != nil {
		services = BoundServiceNamesFromSecret(aux.services)
	}

	// II. Unpack the core application resource

	origin, err := Origin(&appCR)
	if err != nil {
		return nil, errors.Wrap(err, "finding origin")
	}

	chartName, err := AppChart(&appCR)
	if err != nil {
		return nil, errors.Wrap(err, "finding app chart")
	}

	stageID, err := StageID(&appCR)
	if err != nil {
		return nil, errors.Wrap(err, "finding the stage id")
	}

	imageURL, err := ImageURL(&appCR)
	if err != nil {
		return nil, errors.Wrap(err, "finding the image url")
	}

	builderURL, err := BuilderURL(&appCR)
	if err != nil {
		return nil, errors.Wrap(err, "finding the builder url")
	}

	settings, err := Settings(&appCR)
	if err != nil {
		return nil, errors.Wrap(err, "finding settings")
	}

	desiredRoutes, err := DesiredRoutes(&appCR)
	if err != nil {
		return nil, errors.Wrap(err, "finding desired routes")
	}

	// III. Assemble the main structure

	meta := models.NewAppRef(appName, namespace)
	app := meta.App()

	app.Meta.CreatedAt = appCR.GetCreationTimestamp()

	app.Configuration.Instances = &instances
	app.Configuration.Configurations = configurations
	app.Configuration.Environment = environment
	app.Configuration.Services = services
	app.Configuration.Routes = desiredRoutes
	app.Configuration.AppChart = chartName
	app.Configuration.Settings = settings
	app.Origin = origin
	app.StageID = stageID
	app.ImageURL = imageURL
	app.Staging.Builder = builderURL

	// IV. Assemble the deployment structure for active applications.

	podMetrics := []metricsv1beta1.PodMetrics{}

	if metrics != nil {
		// extract the metrics for the app, based on the app pods
		for _, pod := range appPods {
			m, found := metrics[pod.Name]
			if found {
				podMetrics = append(podMetrics, m)
			}
		}
	}

	app.Workload, err = NewWorkload(cluster, app.Meta, instances).
		AssembleFromParts(ctx, appPods, podMetrics, appRoutes)
	if err != nil {
		return nil, err
	}

	// set app status and done ...

	app.StagingStatus = aux.staging

	if aux.staging == models.ApplicationStagingActive {
		app.Status = models.ApplicationStaging
		return app, nil
	}

	if app.Workload == nil {
		app.Status = models.ApplicationCreated
		return app, nil
	}

	app.Status = models.ApplicationRunning
	return app, nil
}

// fetch is a helper for Lookup. It fetches all information about an application from the cluster.
func fetch(ctx context.Context, cluster *kubernetes.Cluster, app *models.App) error {
	// Consider delayed loading, i.e. on first access, or for transfer (API response).
	// Consider objects for the information which hide the defered loading.  These
	// could also have the necessary modifier methods.  See sibling files scale.go,
	// env.go, configurations.go, ingresses.go.  Defered at the moment, the PR is big
	// enough already.

	// TODO: Check which of the called functions retrieve the CR also. Pass them the
	// CR loaded here to avoid superfluous kube api calls.
	applicationCR, err := Get(ctx, cluster, app.Meta)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown("application resource is missing")
		}

		err = apierror.InternalError(err, "failed to get the application resource")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	desiredRoutes, err := DesiredRoutes(applicationCR)
	if err != nil {
		err = errors.Wrap(err, "finding desired routes")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	origin, err := Origin(applicationCR)
	if err != nil {
		err = errors.Wrap(err, "finding origin")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	environment, err := Environment(ctx, cluster, app.Meta)
	if err != nil {
		err = errors.Wrap(err, "finding env")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	instances, err := Scaling(ctx, cluster, app.Meta)
	if err != nil {
		err = errors.Wrap(err, "finding scaling")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	configurations, err := BoundConfigurationNames(ctx, cluster, app.Meta)
	if err != nil {
		err = errors.Wrap(err, "finding configurations")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	services, err := BoundServiceNames(ctx, cluster, app.Meta)
	if err != nil {
		err = errors.Wrap(err, "finding services")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	chartName, err := AppChart(applicationCR)
	if err != nil {
		err = errors.Wrap(err, "finding app chart")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	stageID, err := StageID(applicationCR)
	if err != nil {
		err = errors.Wrap(err, "finding the stage id")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	imageURL, err := ImageURL(applicationCR)
	if err != nil {
		err = errors.Wrap(err, "finding the image url")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	builderURL, err := BuilderURL(applicationCR)
	if err != nil {
		err = errors.Wrap(err, "finding the builder url")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	settings, err := Settings(applicationCR)
	if err != nil {
		err = errors.Wrap(err, "finding settings")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	app.Meta.CreatedAt = applicationCR.GetCreationTimestamp()

	app.Configuration.Instances = &instances
	app.Configuration.Configurations = configurations
	app.Configuration.Environment = environment
	app.Configuration.Services = services
	app.Configuration.Routes = desiredRoutes
	app.Configuration.AppChart = chartName
	app.Configuration.Settings = settings
	app.Origin = origin
	app.StageID = stageID
	app.ImageURL = imageURL
	app.Staging.Builder = builderURL

	// Check if app is active, and if yes, fill the associated parts.  May have to
	// straighten the workload structure a bit further.

	app.Workload, err = NewWorkload(cluster, app.Meta, instances).Get(ctx)
	if err != nil {
		err = errors.Wrap(err, "workload loading")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	staging, err := stagingStatus(ctx, cluster, app.Meta.Namespace, app.Meta.Name)
	if err != nil {
		err = errors.Wrap(err, "staging app")
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
		return err
	}

	app.StagingStatus = staging[EncodeConfigurationKey(app.Meta.Name, app.Meta.Namespace)]

	if app.StagingStatus == models.ApplicationStagingActive {
		app.Status = models.ApplicationStaging
		return nil
	}

	if app.Workload == nil {
		app.Status = models.ApplicationCreated
		return nil
	}

	app.Status = models.ApplicationRunning
	return nil
}

// getTailLines returns the number of log lines to tail based on LOG_TAIL_LINES env var
func getTailLines() *int64 {
	if val := os.Getenv("LOG_TAIL_LINES"); val != "" {
		if lines, err := strconv.ParseInt(val, 10, 64); err == nil {
			return &lines
		}
	}
	return nil
}
