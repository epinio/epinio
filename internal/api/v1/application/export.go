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
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/internal/urlcache"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// ExportToRegistry handles the API endpoint GET /namespaces/:namespace/applications/:app/export
// It exports the named application to the export registry (also specified by name)
func ExportToRegistry(c *gin.Context) apierror.APIErrors {
	// //////////////////////////////////////////////////////////////////////////////
	/// Validate request, and fill in defaults where needed

	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	logger := requestctx.Logger(ctx)

	req := models.AppExportRequest{}
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequestError(err.Error()).
			WithDetails("failed to unmarshal app export request")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	// Get application
	theApp, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	// destination validation I - do we have a name ?
	if req.Destination == "" {
		return apierror.NewBadRequestError("export destination is missing/empty")
	}

	destination, certSecretName, err := checkDestination(ctx, cluster, req.Destination)
	if err != nil {
		return apierror.InternalError(err)
	}
	logger.Info("OCI export", "destination", destination.URL, "certs@", certSecretName)

	destinationURL, err := decodeDestination(req.Destination, destination.URL)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("OCI export destination", "url scheme", destinationURL.Scheme)
	logger.Info("OCI export destination", "url host", destinationURL.Host)
	logger.Info("OCI export destination", "url path", destinationURL.Path)

	// Make the certs available as file for use by registry clients, if any.
	certFile := ""
	if certSecretName != "" {
		certFile, err = loadCerts(ctx, cluster, certSecretName)
		if err != nil {
			return apierror.InternalError(err)
		}
		defer func(path string) {
			cleanupLocalPath(logger, "cert", path)
		}(certFile)
	}

	logger.Info("OCI export transport", "cert file", certFile)

	applyDefaults(&req, namespace, appName, theApp.StageID)

	// //////////////////////////////////////////////////////////////////////////////
	/// Helper information
	//
	// ATTENTION: We may see multiple requests for export of the same application, at near the
	// same time. Using unix nanoseconds as unique file base name to separate these in the file
	// system.

	// NOTE: Using namespace and appname in the file name is considered by the security checker
	// as a means of introducing path injection attacks. Not true actually, as the names are
	// restricted by the kube regexes. Still easier to just use the timestamp.

	trimmedDestination := trimSchemes(destination.URL)

	base := fmt.Sprintf("oci-export-%d", time.Now().UnixNano())

	chartLocalFile := base + "-chart.tar.gz"
	imageLocalFile := base + "-image.tar"
	imageRemoteFile := trimmedDestination + "/" + req.ImageName + ":" + req.ImageTag

	logger.Info("OCI export local chart", "path", chartLocalFile)
	logger.Info("OCI export local image", "path", imageLocalFile)
	logger.Info("OCI export remote image", "path", imageRemoteFile)

	// //////////////////////////////////////////////////////////////////////////////
	/// Retrieve the app data and save it to local files.
	//
	// The base destination path is (const `imageExportVolume`)
	// See sibling file `part.go` for the definition.
	// This is the directory for export files, as seen from here, the API server.
	//
	// ATTENTION: The image upload job sees the file at a different base path, as per the volume
	//           and mount declarations.

	logger.Info("OCI export fetch chart archive", "path", chartLocalFile)
	apierr := fetchAppChartFile(ctx, logger, cluster, theApp, chartLocalFile)
	if apierr != nil {
		return apierr
	}
	defer func(path string) {
		cleanupLocalPath(logger, "chart", path)
	}(imageExportVolume + chartLocalFile)
	// Note: By passing the string as argument instead of through the closure we can change
	// chartLocalFile without affecting what file is removed by the deferal.

	client, err := helm.GetHelmClient(cluster.RestConfig, logger, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("OCI export fetch values")

	values, err := client.GetReleaseValues(names.ReleaseName(theApp.Meta.Name), false)
	if err != nil {
		return apierror.InternalError(err)
	}

	// And fetch image ...

	logger.Info("OCI export fetch image", "path", imageLocalFile,
		"origin", "docker://"+theApp.ImageURL)
	imageFile, err := fetchAppImageFile(ctx, logger, cluster, theApp, imageLocalFile)
	if err != nil {
		return apierror.InternalError(err)
	}
	imageFile.Close() // Note: We do not need the open reader here.
	defer func(path string) {
		cleanupLocalPath(logger, "image", path)
	}(imageExportVolume + imageLocalFile)

	// //////////////////////////////////////////////////////////////////////////////
	///
	// Rewrite the chart for the desired name/tag. Also integrate the app configuration,
	// i.e. the data from app's `values.yaml`. As part of that insert the proper image
	// reference too.

	values["epinio"].(map[string]interface{})["imageURL"] = imageRemoteFile

	tmp, chartLocalFile, err := rewriteChart(logger,
		imageExportVolume+chartLocalFile,
		req.ChartName, req.ChartVersion, values)
	if tmp != "" {
		// This removes the tarball as well, as it resides in tmp.
		defer func(path string) {
			cleanupLocalPath(logger, "chart repack dir", path)
		}(tmp)
	}
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("OCI export rewritten chart", "chart", chartLocalFile)

	// //////////////////////////////////////////////////////////////////////////////
	/// Upload the parts to the chosen destination

	// chart archive ...

	destinationHost := destinationURL.Host

	logger.Info("OCI export login", "host'", destinationHost)

	rOpts := []action.RegistryLoginOpt{}
	if certFile != "" {
		logger.Info("OCI export login", "+certs", certFile)
		rOpts = append(rOpts, action.WithCAFile(certFile))
	}

	err = client.RegistryLogin(destinationHost, destination.Username, destination.Password, rOpts...)
	if err != nil {
		return apierror.InternalError(err)
	}

	ociRegistry := destination.URL
	if !strings.HasPrefix(ociRegistry, "oci://") {
		ociRegistry = "oci://" + trimmedDestination
	}

	logger.Info("OCI export push", "registry", ociRegistry, "from", chartLocalFile)

	pOpts := []action.PushOpt{}
	if certFile != "" {
		logger.Info("OCI export push", "+certs", certFile)
		pOpts = append(pOpts, action.WithTLSClientConfig("", "", certFile))
	}
	_, err = client.Push(chartLocalFile, ociRegistry, pOpts...)
	// NOTE: Neither chart name nor version are specified here!
	// See `rewriteChart` above for the place where this information is inserted.

	logger.Info("OCI export upload chart error", "err", err)
	if err != nil {
		return apierror.InternalError(err)
	}

	// image ...

	imageJob := createCopyJob(logger,
		"docker-archive:/workspace/"+imageLocalFile,
		// NOTE: `/workspace` is the `imageExportVolume` as seen from the job.
		"docker://"+imageRemoteFile,
		req.Destination, certSecretName)

	err = runJob("image push", ctx, cluster, logger, imageJob)
	if err != nil {
		return apierror.InternalError(err)
	}

	// //////////////////////////////////////////////////////////////////////////////
	response.OK(c)
	return nil
}

func rewriteChart(logger logr.Logger, path, name, version string, params map[string]interface{}) (string, string, error) {
	logger.Info("OCI export load chart", "chart", path)

	tmpDir, err := os.MkdirTemp("", "oci-export")
	if err != nil {
		return "", "", errors.Wrap(err, "can't create temp directory")
	}

	appChart, err := loader.Load(path)
	if err != nil {
		return tmpDir, "", err
	}

	logger.Info("OCI export chart meta", "chart", appChart.Metadata)
	logger.Info("OCI export chart values", "chart", appChart.Values)

	logger.Info("OCI export rewrite chart", "name", name, "version", version)
	logger.Info("OCI export rewrite chart", "values", params)

	appChart.Metadata.Name = name
	appChart.Metadata.Version = version

	merged := chartutil.CoalesceTables(params, appChart.Values)

	// ATTENTION: While we can directly edit the meta data and see it in the saved tarball the
	// handling of the values is more complex. The `appChart.Values` is a read-only convenience
	// field. To make changes it is necessary to edit the `Raw` slice holding the actual file
	// contents, including the `values.yaml`. Being a slice we have to do a linear search :( And
	// as the app chart may be a user-specified thing, not our standard consider the possibility
	// that it does not have a `values.yaml`.

	yaml, err := yaml.Marshal(merged)
	if err != nil {
		return tmpDir, "", err
	}
	ok := false
	for index, file := range appChart.Raw {
		if file.Name == chartutil.ValuesfileName {
			appChart.Raw[index].Data = yaml
			ok = true
			break
		}
	}
	if !ok {
		// Values file missing, create it.
		appChart.Raw = append(appChart.Raw, &chart.File{
			Name: chartutil.ValuesfileName,
			Data: yaml,
		})
	}

	logger.Info("OCI export rewritten chart meta", "chart", appChart.Metadata)
	logger.Info("OCI export rewritten chart values", "YAML", string(yaml))

	logger.Info("OCI export validate chart")
	err = appChart.Validate()
	if err != nil {
		return tmpDir, "", err
	}

	logger.Info("OCI export save chart", "tmp", tmpDir)
	tarball, err := chartutil.Save(appChart, tmpDir)
	return tmpDir, tarball, err
}

func applyDefaults(req *models.AppExportRequest, namespace, appName, stageID string) {
	base := fmt.Sprintf("%s-%s", namespace, appName)

	if req.ImageName == "" {
		req.ImageName = base + "-image"
	}
	if req.ChartName == "" {
		req.ChartName = base + "-chart"
	}
	if req.ImageTag == "" {
		req.ImageTag = stageID
	}
	if req.ChartVersion == "" {
		req.ChartVersion = "0.0.0"
	}
}

func checkDestination(ctx context.Context, cluster *kubernetes.Cluster,
	destination string) (registry.RegistryCredentials, string, error) {

	empty := registry.RegistryCredentials{}

	// destination validation II - do we have a secret for the name ?
	destinationSecret, err := cluster.GetSecret(ctx, helmchart.Namespace(), destination)
	if err != nil {
		return empty, "", errors.Wrap(err, "bad export destination")
	}

	// destination validation III - is the secret good ?
	marker, ok := destinationSecret.ObjectMeta.Labels[kubernetes.EpinioAPIExportRegistryLabelKey]
	if !ok {
		return empty, "", errors.New("bad export destination: marker label is missing")
	}

	if marker != "true" {
		return empty, "", errors.New("bad export destination: bad value for marker label")
	}

	creds, err := registry.GetRegistryCredentialsFromSecret(*destinationSecret)
	if err != nil {
		return empty, "", errors.Wrap(err, "bad export destination: bad url")
	}

	// Check for and use a cert secret sibling to the auth secret.

	certSecretName := ""
	certSecret, ok := destinationSecret.Data["certs"]
	if ok {
		certSecretName = string(certSecret)
	}

	return creds, certSecretName, nil
}

// cleanupLocalPath is invoked in deferals to remove the temporary files and directories used by an
// export after they are not required any longer.
func cleanupLocalPath(logger logr.Logger, label, path string) {
	logger.Info("OCI export cleanup local "+label, "path", path)
	err := os.RemoveAll(path)
	if err != nil {
		logger.Error(fmt.Errorf("error cleaning up local %s", label),
			"path", path,
			"error", err.Error())
	}
}

// ATTENTION TODO Compare `fetchAppChart` (see `part.go`), DRY them.
func fetchAppChartFile(ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster,
	theApp *models.App, destinationPath string) apierror.APIErrors {
	// Get the application's app chart
	appChart, err := appchart.Lookup(ctx, cluster, theApp.Configuration.AppChart)
	if err != nil {
		return apierror.InternalError(err)
	}
	if appChart == nil {
		return apierror.AppChartIsNotKnown(theApp.Configuration.AppChart)
	}

	chartArchive, err := chartArchiveURL(appChart, cluster.RestConfig, logger)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("input", "chart-url", chartArchive)

	chartArchive, err = urlcache.Get(ctx, logger, chartArchive)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("input", "chart-file", chartArchive)

	// Here the archive is surely a local file

	file, err := os.Open(chartArchive)
	if err != nil {
		return apierror.InternalError(err)
	}
	defer file.Close()

	logger.Info("input is file")

	dstFile, err := os.Create(imageExportVolume + destinationPath)
	if err != nil {
		return apierror.InternalError(err)
	}
	defer dstFile.Close()

	// copy file ...
	logger.Info("input, copy to", "destination", dstFile.Name())

	_, err = io.Copy(dstFile, file)
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}

func createCopyJob(logger logr.Logger, localPath, destinationPath, authSecret, certSecret string) *batchv1.Job {
	// See also part.go, runDownloadImageJob - Look into DRY'ing

	nano := fmt.Sprintf("%d", time.Now().UnixNano())
	jobName := names.GenerateResourceName("oci-push-image", nano)

	appImageExporter := viper.GetString("app-image-exporter")

	labels := map[string]string{
		"app.kubernetes.io/name":       names.Truncate(jobName, 63),
		"app.kubernetes.io/part-of":    helmchart.Namespace(),
		"app.kubernetes.io/managed-by": "epinio",
		"app.kubernetes.io/component":  "export",
	}

	volumes := []corev1.Volume{{
		Name: "image-export-volume",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "image-export-pvc",
			},
		},
	}, {
		Name: "registry-creds-volume",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: authSecret,
				Items: []corev1.KeyToPath{
					{
						Key:  ".dockerconfigjson",
						Path: "auth.json",
					},
				},
			},
		},
	}}

	mounts := []corev1.VolumeMount{{
		Name:      "image-export-volume",
		MountPath: "/workspace/",
	}, {
		Name:      "registry-creds-volume",
		MountPath: "/root/containers/",
	}}

	if certSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "registry-cert-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: certSecret,
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "registry-cert-volume",
			MountPath: "/etc/ssl/certs/",
		})
	}

	args := []string{
		"copy",
		"--dest-authfile=/root/containers/auth.json",
		localPath,
		destinationPath,
	}

	logger.Info("OCI export image copy command", "skopeo", args)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        jobName,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app.kubernetes.io/name",
												Operator: "In",
												Values:   []string{"epinio-server"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:         "oci-push",
							Image:        appImageExporter,
							Command:      []string{"skopeo"},
							Args:         args,
							VolumeMounts: mounts,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       volumes,
				},
			},
		},
	}

	return job
}

// runJob executes the given kube job and waits for its completion (or timeout (12 minutes (**))).
// Note that this is a generic function which may be useful in other contexts.  In that case it
// should be moved to the kubernetes section of the helpers package.
//
// (**) In local testing 4 minutes were seen for job completion.
//
// See also part.go, getFileImageAndJobCleanup - Look into DRY'ing
func runJob(label string, ctx context.Context, cluster *kubernetes.Cluster, logger logr.Logger, job *batchv1.Job) error {
	logger.Info(fmt.Sprintf("run %s job", label))

	err := cluster.CreateJob(ctx, helmchart.Namespace(), job)
	if err != nil {
		logger.Error(err, "job create", job.ObjectMeta.Name)
		return errors.Wrapf(err, "unable to create %s job %s", label, job.ObjectMeta.Name)
	}

	logger.Info(fmt.Sprintf("wait for completion of %s job", label))

	err = cluster.WaitForJobDone(ctx, helmchart.Namespace(), job.ObjectMeta.Name, time.Minute*12)
	if err != nil {
		logger.Error(err, "job wait", job.ObjectMeta.Name)
		return errors.Wrapf(err, "error waiting for completion of %s job %s", label, job.ObjectMeta.Name)
	}

	failed, err := cluster.IsJobFailed(ctx, job.ObjectMeta.Name, helmchart.Namespace())
	if err != nil {
		logger.Error(err, "job", job.ObjectMeta.Name)
		return errors.Wrapf(err, "error checking status of %s job %s", label, job.ObjectMeta.Name)
	}

	if failed {
		logger.Info("job failed", "job", job.ObjectMeta.Name)
		return errors.New(label + " job " + job.ObjectMeta.Name + " failed")
	} else {
		// Attention: Job is deleted if and only if it succeeded in time. A failed or timed
		// out job is kept for inspection by the user and/or operator.

		logger.Info(fmt.Sprintf("delete completed %s job %s", label, job.ObjectMeta.Name))
		err = cluster.DeleteJob(ctx, helmchart.Namespace(), job.ObjectMeta.Name)
		if err != nil {
			logger.Error(err, "job", job.ObjectMeta.Name)
			return errors.Wrapf(err, "error deleting %s job %s", label, job.ObjectMeta.Name)
		}
	}

	return nil
}

func loadCerts(ctx context.Context, cluster *kubernetes.Cluster, secretName string) (string, error) {
	cert, err := cluster.GetSecret(ctx, helmchart.Namespace(), secretName)
	if err != nil {
		return "", err
	}

	pemData, ok := cert.Data["tls.crt"]
	if !ok {
		return "", fmt.Errorf("Cert secret %s not suitable, no key 'tls.crt'", secretName)
	}

	certFile := fmt.Sprintf("%soci-cert-%d.pem", imageExportVolume, time.Now().UnixNano())

	err = os.WriteFile(certFile, pemData, 0600)
	if err != nil {
		return "", err
	}

	return certFile, nil
}

func trimSchemes(url string) string {
	url = strings.TrimPrefix(url, "oci://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	return url
}

func decodeDestination(name, destination string) (*url.URL, error) {
	// `destination` is an url coming out of the host reference field of a docker config.json file.
	// It has to contain at least a hostname.
	//
	// Examples:
	// (1) https://index.docker.io/v1/
	// (2) registry.suse.com
	// (3) 172-18-0-5.nip.io:5000
	// (4) index.docker.io/v1/
	//
	// Note that the url may or may not have a leading schema.
	// Further note that examples (3) and (4) are actually __not__ proper urls.
	// Yet they are acceptable as keys in a docker config.json file.
	// Example (3) will fail to parse.
	// Example (4) will mis-identify the hostname part as part of the url path.
	// Both are cured by adding a schema.
	//
	// On the other side while a string like `/172-18-0-5.nip.io:5000/foo` is an url it is not
	// valid here, as it does not have a host reference.
	//
	// See https://github.com/golang/go/issues/18824 for more notes.

	rawDest := destination
	if !strings.Contains(destination, "://") {
		// No scheme present. This will lead to a number of parsing issues.
		// Force a scheme.
		destination = "oci://" + destination
	}

	destinationURL, err := url.Parse(destination)
	if err != nil {
		return nil, err
	}

	if destinationURL.Host == "" {
		// No host present. That is an error.
		return nil, fmt.Errorf("Registry '%s': Missing host in '%s'", name, rawDest)
	}

	return destinationURL, nil
}
