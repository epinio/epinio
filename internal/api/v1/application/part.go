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
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
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
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/repo"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
)

// GetPart handles the API endpoint GET /namespaces/:namespace/applications/:app/part/:part
// It determines the contents of the requested part (values, chart, image) and returns as
// the response of the handler.
func GetPart(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	partName := c.Param("part")
	logger := requestctx.Logger(ctx)

	switch partName {
	case "manifest", "values", "chart", "image":
		// valid parts, no error
	default:
		return apierror.NewBadRequestErrorf("unknown '%s' part, expected chart, manifest, image, or values", partName)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	if partName == "manifest" {
		return fetchAppManifest(c, app)
	}

	// While the app exists it has no workload, and therefore no chart/image/values to
	// export. Manifest however is fine, see above for its handler.

	if app.Workload == nil {
		return apierror.NewBadRequestError("no chart available for application without workload")
	}

	switch partName {
	case "chart":
		return fetchAppChart(c, ctx, logger, cluster, app.Meta)
	case "image":
		return fetchAppImage(c, ctx, logger, cluster, app.Meta)
	case "values":
		return fetchAppValues(c, logger, cluster, app.Meta)
	}

	return apierror.InternalError(fmt.Errorf("should not be reached"))
}

func fetchAppChart(c *gin.Context, ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster, app models.AppRef) apierror.APIErrors {

	// Get application
	theApp, err := application.Lookup(ctx, cluster, app.Namespace, app.Name)
	if err != nil {
		return apierror.InternalError(err)
	}

	if theApp == nil {
		return apierror.AppIsNotKnown(app.Name)
	}

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

	logger.Info("input", "chart archive", chartArchive)

	// Try to read the archive as local path first, before falling back to retrieval via http.

	file, err := os.Open(chartArchive)
	if err == nil {
		logger.Info("input", "chart archive", "is file")

		fileInfo, err := file.Stat()
		if err == nil {
			logger.Info("input", "chart archive", "has stat")

			contentLength := fileInfo.Size()
			contentType := "application/x-gzip"

			logger.Info("input", "chart archive", "returning file")

			c.DataFromReader(http.StatusOK, contentLength, contentType, bufio.NewReader(file),
				map[string]string{
					"X-Content-Length": strconv.FormatInt(contentLength, 10),
				})

			return nil
		}
	}

	logger.Info("input", "chart archive", "retrieving by http")

	response, err := http.Get(chartArchive) // nolint:gosec // app chart repo ref
	if err != nil || response.StatusCode != http.StatusOK {
		logger.Info("fail, http issue")

		c.Status(http.StatusServiceUnavailable)
		return nil
	}

	reader := response.Body
	contentLength := response.ContentLength
	contentType := response.Header.Get("Content-Type")

	logger.Info("OK",
		"origin", c.Request.URL.String(),
		"returning", fmt.Sprintf("%d bytes %s as is", contentLength, contentType),
	)
	c.DataFromReader(http.StatusOK, contentLength, contentType, reader, map[string]string{
		"X-Content-Length": strconv.FormatInt(contentLength, 10),
	})
	return nil
}

func fetchAppImage(c *gin.Context, ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster, appRef models.AppRef) apierror.APIErrors {
	logger.Info("fetching app image")

	// Get application
	theApp, err := application.Lookup(ctx, cluster, appRef.Namespace, appRef.Name)
	if err != nil {
		return apierror.InternalError(err)
	}

	jobName := names.GenerateResourceName("image-export-job", appRef.Namespace, appRef.Name, theApp.StageID)
	imageOutputFilename := fmt.Sprintf("%s-%s-%s.tar", appRef.Namespace, appRef.Name, theApp.StageID)

	logger.Info("got app chart", "chart image", theApp.ImageURL)

	err = runDownloadImageJob(ctx, cluster, jobName, theApp.ImageURL, imageOutputFilename)
	if err != nil {
		return apierror.NewInternalError("failed to create job", "error", err.Error())
	}

	file, err := getFileImageAndJobCleanup(ctx, cluster, jobName, imageOutputFilename)
	if err != nil {
		return apierror.NewInternalError("failed waiting for job done", "error", err.Error())
	}

	defer func() {
		err := os.Remove("/image-export/" + imageOutputFilename)
		if err != nil {
			logger.Info("error cleaning up image file", "filename", imageOutputFilename, "error", err.Error())
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return apierror.NewInternalError("failed to get file info", "error", err.Error())
	}

	c.DataFromReader(http.StatusOK, fileInfo.Size(), "application/x-tar", bufio.NewReader(file), map[string]string{
		"X-Content-Length": strconv.FormatInt(fileInfo.Size(), 10),
	})

	return nil
}

func runDownloadImageJob(ctx context.Context, cluster *kubernetes.Cluster, jobName, imageURL, imageOutputFilename string) error {
	appImageExporter := viper.GetString("app-image-exporter")

	labels := map[string]string{
		"app.kubernetes.io/name":       names.Truncate(jobName, 63),
		"app.kubernetes.io/part-of":    helmchart.Namespace(),
		"app.kubernetes.io/managed-by": "epinio",
		"app.kubernetes.io/component":  "staging",
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
				SecretName: registry.CredentialsSecretName,
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
		MountPath: "/tmp/",
	}, {
		Name:      "registry-creds-volume",
		MountPath: "/root/containers/",
	}}

	registryCertificateSecret := viper.GetString("registry-certificate-secret")
	if registryCertificateSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "registry-cert-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: registryCertificateSecret,
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "registry-cert-volume",
			MountPath: "/etc/ssl/certs/",
		})
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        jobName,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: pointer.Int32(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "skopeo",
							Image:   appImageExporter,
							Command: []string{"skopeo"},
							Args: []string{
								"copy",
								"--src-authfile=/root/containers/auth.json",
								"docker://" + imageURL,
								"docker-archive:/tmp/" + imageOutputFilename,
							},
							VolumeMounts: mounts,
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       volumes,
				},
			},
		},
	}

	return cluster.CreateJob(ctx, helmchart.Namespace(), job)
}

func getFileImageAndJobCleanup(ctx context.Context, cluster *kubernetes.Cluster, jobName, imageOutputFilename string) (*os.File, error) {
	err := cluster.WaitForJobDone(ctx, helmchart.Namespace(), jobName, time.Minute*2)
	if err != nil {
		return nil, errors.Wrapf(err, "error waiting for job done %s", jobName)
	}

	// check for file existence
	file, err := os.Open("/image-export/" + imageOutputFilename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open tar file")
	}

	err = cluster.DeleteJob(ctx, helmchart.Namespace(), jobName)

	return file, errors.Wrapf(err, "error deleting job %s", jobName)

}

func fetchAppValues(c *gin.Context, logger logr.Logger, cluster *kubernetes.Cluster, app models.AppRef) apierror.APIErrors {
	yaml, err := helm.Values(cluster, logger, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKBytes(c, yaml)
	return nil
}

func fetchAppManifest(c *gin.Context, app *models.App) apierror.APIErrors {
	m := models.ApplicationManifest{
		ApplicationCreateRequest: models.ApplicationCreateRequest{
			Name:          app.Meta.Name,
			Configuration: app.Configuration,
		},
		Namespace: app.Meta.Namespace,
		Origin:    app.Origin,
		Staging:   app.Staging,
	}

	response.OKYaml(c, m)
	return nil
}

// chartArchiveURL returns a url for the helm chart's tarball.
//
// The chart is specified as simple name, and resolved to actual archive through a helm repo. This
// code is a __HACK__. At various levels.
//
// We create and initialize a mittwald client, this gives us the basic dir structure needed.
//
// We add the repository needed. This gives us the chart and index files for it in the above
// directory hierarchy.
//
// We do __NOT__ use a low-level helm puller action. Even setting it up with configuration and
// settings of the above client it will look in the wrong place for the repo index. I.e. looks to
// completely ignore the RepositoryCache setting.
//
// So, to continue the hack, we access the repo index.yaml directly, i.e. read in, unmarshal into
// minimally required structure and then locate the chart and its urls.
//
// The advantage of this hack: We get a fetchable url we can feed into the part invoked when the
// chart was specified as direct url. No going through a temp file.
func chartArchiveURL(c *models.AppChartFull, restConfig *restclient.Config, logger logr.Logger) (string, error) {
	if c.HelmRepo == "" {
		return c.HelmChart, nil
	}

	helmChart := c.HelmChart
	helmVersion := ""

	// Split chart ref into name and version
	pieces := strings.SplitN(helmChart, ":", 2)
	if len(pieces) == 2 {
		helmVersion = pieces[1]
		helmChart = pieces[0]
	}

	// Set up client and repo, ensures proper directory structure and presence of index file
	name := names.GenerateResourceName("hr-" + base64.StdEncoding.EncodeToString([]byte(c.HelmRepo)))

	client, err := helm.GetHelmClient(restConfig, logger, "")
	if err != nil {
		return "", err
	}

	if err := client.AddOrUpdateChartRepo(repo.Entry{
		Name: name,
		URL:  c.HelmRepo,
	}); err != nil {
		return "", err
	}

	// Read index into memory
	content, err := os.ReadFile("/tmp/.helmcache/" + name + "-index.yaml")
	if err != nil {
		return "", err
	}

	// Get minimal structure need to locate the chart by name, and version.
	var index struct {
		Entries map[string][]struct {
			Version string   `yaml:"version"`
			URLs    []string `yaml:"urls"`
		} `yaml:"entries"`
	}

	err = yaml.Unmarshal(content, &index)
	if err != nil {
		return "", err
	}

	entries, ok := index.Entries[helmChart]
	if !ok {
		return "", errors.New("Chart '" + helmChart + "' not found")
	}

	for _, entry := range entries {
		// If no version is specified, take the first found.  Assumes that
		// first in sequence is highest version, i.e. latest.
		if helmVersion == "" || entry.Version == helmVersion {
			return entry.URLs[0], nil
		}
	}

	return "", errors.New("Chart '" + helmChart + "' version '" + helmVersion + "' not found")
}
