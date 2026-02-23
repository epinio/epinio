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
	"archive/zip"
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/internal/urlcache"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

const imageExportVolume = "/image-export/"

// validPartNames lists part names accepted by GetPart (manifest, values, chart, image, archive).
var validPartNames = []string{"manifest", "values", "chart", "image", "archive"}

func isValidPartName(part string) bool {
	for _, p := range validPartNames {
		if part == p {
			return true
		}
	}
	return false
}

// Has to match mount path of `image-export-volume` in templates/server.yaml of the chart
// CONSIDER ? Templated, and name given to server through EV ?

// GetPart handles the API endpoint GET /namespaces/:namespace/applications/:app/part/:part
// It determines the contents of the requested part (values, chart, image) and returns as
// the response of the handler.
func GetPart(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	partName := c.Param("part")

	if !isValidPartName(partName) {
		return apierror.NewBadRequestErrorf("unknown '%s' part, expected chart, manifest, image, values, or archive", partName)
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
		return fetchAppChart(c, ctx, cluster, app)
	case "image":
		return fetchAppImage(c, ctx, cluster, app)
	case "values":
		return fetchAppValues(c, cluster, app.Meta)
	case "archive":
		return fetchAppArchive(c, ctx, cluster, app)
	}

	return apierror.InternalError(fmt.Errorf("should not be reached"))
}

// ATTENTION TODO Compare `fetchAppChartFile` (see `export.go`), DRY them.

func fetchAppChart(
	c *gin.Context,
	ctx context.Context,
	cluster *kubernetes.Cluster,
	theApp *models.App,
) apierror.APIErrors {
	log := requestctx.Logger(ctx)
	// Get the application's app chart
	appChart, err := appchart.Lookup(ctx, cluster, theApp.Configuration.AppChart)
	if err != nil {
		return apierror.InternalError(err)
	}
	if appChart == nil {
		return apierror.AppChartIsNotKnown(theApp.Configuration.AppChart)
	}

	chartArchive, err := chartArchiveURL(appChart, cluster.RestConfig)
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow("input", "chart archive", chartArchive)

	// Ensure presence of the chart archive as a local file.

	chartArchive, err = urlcache.Get(ctx, chartArchive)
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow("input", "local chart archive", chartArchive)

	// Here the archive is surely a local file

	file, err := os.Open(chartArchive) // nolint:gosec // path from urlcache under controlled export volume
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow("input is file")

	fileInfo, err := file.Stat()
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow("input has stat")

	contentLength := fileInfo.Size()
	contentType := "application/x-gzip"

	log.Infow("input, returning file")

	log.Infow("OK",
		"origin", c.Request.URL.String(),
		"returning", fmt.Sprintf("%d bytes %s as is", contentLength, contentType),
	)

	c.DataFromReader(http.StatusOK, contentLength, contentType, bufio.NewReader(file), nil)
	return nil
}

func fetchAppImage(
	c *gin.Context,
	ctx context.Context,
	cluster *kubernetes.Cluster,
	theApp *models.App,
) apierror.APIErrors {
	log := requestctx.Logger(ctx)
	log.Infow("fetching app image")

	// Mixing in nanoseconds to prevent multiple requests for the same app to clash over the file name
	now := strconv.Itoa(time.Now().Nanosecond())
	imageOutputFilename := fmt.Sprintf(
		"%s-%s-%s-%s.tar",
		theApp.Meta.Namespace,
		theApp.Meta.Name,
		theApp.StageID,
		now,
	)

	log.Infow("got app chart", "chart image", theApp.ImageURL)

	file, err := fetchAppImageFile(ctx, cluster, theApp, imageOutputFilename)
	if err != nil {
		return apierror.NewInternalError("failed to retrieve image", err.Error())
	}

	defer func() {
		err := os.Remove(imageExportVolume + imageOutputFilename)
		if err != nil {
			log.Infow(
				"error cleaning up image file",
				"filename",
				imageOutputFilename,
				"error",
				err.Error(),
			)
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return apierror.NewInternalError("failed to get file info", "error", err.Error())
	}

	c.DataFromReader(http.StatusOK, fileInfo.Size(), "application/x-tar", bufio.NewReader(file), nil)
	return nil
}

// fetchAppArchive streams a zip containing values.yaml, app-chart.tar.gz, and app-image.tar
// so the client can download one archive instead of three parts and zipping in the browser.
func fetchAppArchive(
	c *gin.Context,
	ctx context.Context,
	cluster *kubernetes.Cluster,
	theApp *models.App,
) apierror.APIErrors {
	log := requestctx.Logger(ctx)
	log.Infow("fetching app archive (chart and images)")

	// 1. Values
	valuesYAML, err := helm.Values(cluster, theApp.Meta)
	if err != nil {
		return apierror.InternalError(err)
	}

	// 2. Chart (local path)
	appChart, err := appchart.Lookup(ctx, cluster, theApp.Configuration.AppChart)
	if err != nil {
		return apierror.InternalError(err)
	}
	if appChart == nil {
		return apierror.AppChartIsNotKnown(theApp.Configuration.AppChart)
	}
	chartArchive, err := chartArchiveURL(appChart, cluster.RestConfig)
	if err != nil {
		return apierror.InternalError(err)
	}
	chartArchive, err = urlcache.Get(ctx, chartArchive)
	if err != nil {
		return apierror.InternalError(err)
	}
	chartFile, err := os.Open(chartArchive) // nolint:gosec // path from urlcache under controlled export volume
	if err != nil {
		return apierror.InternalError(err)
	}
	defer func() { _ = chartFile.Close() }()
	chartInfo, err := chartFile.Stat()
	if err != nil {
		return apierror.InternalError(err)
	}

	// 3. Image (job + file on PVC)
	now := strconv.Itoa(time.Now().Nanosecond())
	imageOutputFilename := fmt.Sprintf(
		"%s-%s-%s-%s.tar",
		theApp.Meta.Namespace,
		theApp.Meta.Name,
		theApp.StageID,
		now,
	)
	imageFile, err := fetchAppImageFile(ctx, cluster, theApp, imageOutputFilename)
	if err != nil {
		return apierror.NewInternalError("failed to retrieve image", err.Error())
	}
	defer func() {
		_ = imageFile.Close()
		_ = os.Remove(imageExportVolume + imageOutputFilename)
	}()
	imageInfo, err := imageFile.Stat()
	if err != nil {
		return apierror.NewInternalError("failed to get image file info", err.Error())
	}

	// Stream zip
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", theApp.Meta.Name+"-helm-chart.zip"))
	zw := zip.NewWriter(c.Writer)
	defer func() { _ = zw.Close() }()

	// values.yaml
	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "values.yaml",
		Method: zip.Store,
	})
	if err != nil {
		return apierror.InternalError(err)
	}
	if _, err := w.Write(valuesYAML); err != nil {
		return apierror.InternalError(err)
	}

	// app-chart.tar.gz
	chartSize := chartInfo.Size()
	if chartSize < 0 {
		return apierror.InternalError(fmt.Errorf("invalid chart file size: %d", chartSize))
	}
	w, err = zw.CreateHeader(&zip.FileHeader{
		Name:               "app-chart.tar.gz",
		Method:             zip.Store,
		UncompressedSize64: uint64(chartSize),
	})
	if err != nil {
		return apierror.InternalError(err)
	}
	if _, err := io.Copy(w, chartFile); err != nil {
		return apierror.InternalError(err)
	}

	// app-image.tar
	imageSize := imageInfo.Size()
	if imageSize < 0 {
		return apierror.InternalError(fmt.Errorf("invalid image file size: %d", imageSize))
	}
	w, err = zw.CreateHeader(&zip.FileHeader{
		Name:               "app-image.tar",
		Method:             zip.Store,
		UncompressedSize64: uint64(imageSize),
	})
	if err != nil {
		return apierror.InternalError(err)
	}
	if _, err := io.Copy(w, imageFile); err != nil {
		return apierror.InternalError(err)
	}

	return nil
}

func fetchAppImageFile(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	theApp *models.App,
	imageOutputFilename string,
) (*os.File, error) {
	// Try with source auth first, then retry without source auth in case the auth file
	// does not contain credentials for external registries.
	var lastErr error
	for _, useSourceAuth := range []bool{true, false} {
		if removeErr := os.Remove(imageExportVolume + imageOutputFilename); removeErr != nil && !os.IsNotExist(removeErr) {
			helpers.Logger.Infow("unable to remove stale image export tar before retry", "error", removeErr, "file", imageOutputFilename)
		}

		nano := strconv.Itoa(time.Now().Nanosecond())
		jobName := names.GenerateResourceName(
			"image-export-job",
			theApp.Meta.Namespace,
			theApp.Meta.Name,
			theApp.StageID,
			nano,
		)

		err := runDownloadImageJob(
			ctx,
			cluster,
			jobName,
			theApp.ImageURL,
			imageOutputFilename,
			useSourceAuth,
		)
		if err != nil {
			lastErr = errors.Wrap(err, "unable to create job")
			continue
		}

		file, err := getFileImageAndJobCleanup(
			ctx,
			cluster,
			jobName,
			imageOutputFilename,
		)
		if err == nil {
			return file, nil
		}

		helpers.Logger.Infow(
			"image export attempt failed",
			"job",
			jobName,
			"image",
			theApp.ImageURL,
			"useSourceAuth",
			useSourceAuth,
			"error",
			err,
		)
		lastErr = errors.Wrap(err, "failed waiting for job completion")
	}

	if lastErr == nil {
		lastErr = errors.New("no image export attempts were executed")
	}
	return nil, lastErr
}

func runDownloadImageJob(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	jobName,
	imageURL,
	imageOutputFilename string,
	useSourceAuth bool,
) error {
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

	args := []string{"copy", "--retry-times=5"}
	if useSourceAuth {
		args = append(args, "--src-authfile=/root/containers/auth.json")
	}
	args = append(args, "docker://"+imageURL, "docker-archive:/tmp/"+imageOutputFilename)

	job := newSkopeoJob(jobName, labels, appImageExporter, "skopeo", args, mounts, volumes)

	helpers.Logger.Infow("image export job command", "job", jobName, "args", args)

	return cluster.CreateJob(ctx, helmchart.Namespace(), job)
}

func getFileImageAndJobCleanup(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	jobName,
	imageOutputFilename string,
) (*os.File, error) {
	log := requestctx.Logger(ctx)
	// Allow 15m for slow CI (image pull/skopeo copy can be slow on shared runners)
	err := cluster.WaitForJobDone(ctx, helmchart.Namespace(), jobName, time.Minute*15)
	if err != nil {
		log.Infow("export job wait error", "error", err, "job", jobName)

		if errors.Is(err, context.Canceled) {
			log.Infow("delete job, canceled", "job", jobName)
			// NOTE: Use bg context here, the regular once is canceled.
			err := cluster.DeleteJob(context.Background(), helmchart.Namespace(), jobName)
			if err != nil {
				log.Infow(
					"export job delete error, in cancellation",
					"error",
					err,
					"job",
					jobName,
				)
			}
		}

		return nil, errors.Wrapf(err, "error waiting for job done %s", jobName)
	}

	// Check for file existence (retry because PVC visibility may lag the job completion signal).
	filePath := imageExportVolume + imageOutputFilename
	file, fileErr := openExportImageFileWithRetries(filePath, 10, 2*time.Second)
	if fileErr == nil {
		fileInfo, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			file = nil
			fileErr = errors.Wrap(statErr, "failed to stat image tar file")
		} else if fileInfo.Size() <= 0 {
			_ = file.Close()
			file = nil
			fileErr = errors.New("image export tar file exists but is empty")
		}
	}

	failed, err := cluster.IsJobFailed(ctx, jobName, helmchart.Namespace())
	if err != nil {
		if file != nil {
			helpers.Logger.Infow("unable to check image export job status, but tar file was created", "job", jobName, "error", err)
		} else {
			return nil, errors.Wrapf(err, "error checking job status %s", jobName)
		}
	}
	if failed && file == nil {
		diag := imageExportJobDiagnostics(ctx, cluster, jobName)
		helpers.Logger.Infow("image export job failed", "job", jobName, "diagnostics", diag)
		_ = cluster.DeleteJob(ctx, helmchart.Namespace(), jobName)
		return nil, errors.Errorf("image export job failed (skopeo copy did not produce the tar file): %s", diag)
	}
	if file == nil {
		diag := imageExportJobDiagnostics(ctx, cluster, jobName)
		helpers.Logger.Infow("image export tar missing after completed job", "job", jobName, "diagnostics", diag)
		_ = cluster.DeleteJob(ctx, helmchart.Namespace(), jobName)
		return nil, errors.Wrapf(fileErr, "failed to open tar file (%s)", diag)
	}
	if failed {
		helpers.Logger.Infow("image export job reported failure but tar file exists; continuing with tar artifact", "job", jobName)
	}

	log.Infow("delete job, done", "job", jobName)

	err = cluster.DeleteJob(ctx, helmchart.Namespace(), jobName)
	if err != nil {
		log.Infow("export job delete error", "error", err, "job", jobName)
	}
	return file, errors.Wrapf(err, "error deleting job %s", jobName)
}

func openExportImageFileWithRetries(path string, attempts int, delay time.Duration) (*os.File, error) {
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		file, err := os.Open(path)
		if err == nil {
			return file, nil
		}
		lastErr = err
		if attempt < attempts {
			time.Sleep(delay)
		}
	}

	return nil, lastErr
}

func imageExportJobDiagnostics(ctx context.Context, cluster *kubernetes.Cluster, jobName string) string {
	namespace := helmchart.Namespace()
	diagnostics := []string{}

	job, err := cluster.Kubectl.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("failed to get job %q: %v", jobName, err)
	}

	diagnostics = append(diagnostics, fmt.Sprintf("job counters: active=%d succeeded=%d failed=%d", job.Status.Active, job.Status.Succeeded, job.Status.Failed))
	for _, cond := range job.Status.Conditions {
		diagnostics = append(diagnostics,
			fmt.Sprintf("job condition: type=%s status=%s reason=%s message=%q", cond.Type, cond.Status, cond.Reason, cond.Message))
	}

	pods, err := cluster.ListPods(ctx, namespace, fmt.Sprintf("job-name=%s", jobName))
	if err != nil {
		diagnostics = append(diagnostics, fmt.Sprintf("failed to list job pods: %v", err))
		return strings.Join(diagnostics, " | ")
	}
	if len(pods.Items) == 0 {
		diagnostics = append(diagnostics, "no pods found for job")
		return strings.Join(diagnostics, " | ")
	}

	for _, pod := range pods.Items {
		diagnostics = append(diagnostics,
			fmt.Sprintf("pod %s phase=%s reason=%s message=%q", pod.Name, pod.Status.Phase, pod.Status.Reason, pod.Status.Message))
		for _, st := range pod.Status.ContainerStatuses {
			if st.State.Terminated != nil {
				t := st.State.Terminated
				diagnostics = append(diagnostics,
					fmt.Sprintf("container %s terminated: exitCode=%d reason=%s message=%q", st.Name, t.ExitCode, t.Reason, t.Message))
			} else if st.State.Waiting != nil {
				w := st.State.Waiting
				diagnostics = append(diagnostics,
					fmt.Sprintf("container %s waiting: reason=%s message=%q", st.Name, w.Reason, w.Message))
			}
		}

		logTail, logErr := cluster.Kubectl.CoreV1().Pods(namespace).GetLogs(
			pod.Name,
			&corev1.PodLogOptions{
				Container: "skopeo",
				TailLines: ptr.To[int64](40),
			},
		).DoRaw(ctx)
		if logErr != nil {
			diagnostics = append(diagnostics, fmt.Sprintf("failed to read skopeo logs from pod %s: %v", pod.Name, logErr))
			continue
		}

		cleanLog := strings.ReplaceAll(strings.TrimSpace(string(logTail)), "\n", " || ")
		if cleanLog != "" {
			diagnostics = append(diagnostics, fmt.Sprintf("skopeo log tail from pod %s: %s", pod.Name, cleanLog))
		}
	}

	return strings.Join(diagnostics, " | ")
}

func fetchAppValues(
	c *gin.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
) apierror.APIErrors {
	yaml, err := helm.Values(cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKBytes(c, yaml)
	return nil
}

func fetchAppManifest(c *gin.Context, app *models.App) apierror.APIErrors {
	m := models.ApplicationManifest{
		Name:          app.Meta.Name,
		Configuration: app.Configuration,
		Namespace:     app.Meta.Namespace,
		Origin:        app.Origin,
		Staging:       app.Staging,
	}

	response.OKYaml(c, m)
	return nil
}

// chartArchiveURL returns a url for the helm chart's tarball.
//
// The chart is specified as simple name, and resolved to the actual archive through a helm repo.
// This code is a __HACK__. At various levels.
//
// We create and initialize a mittwald client, this gives us the basic directory structure needed.
//
// We add the repository needed.
// This gives us the chart and index files for it in the above directory hierarchy.
//
// We do __NOT__ use a low-level helm puller action. Even setting it up with configuration and
// settings of the above client it will look in the wrong place for the repo index. I.e. looks to
// completely ignore the RepositoryCache setting.
//
// So, to continue the hack, we access the repostory's index.yaml directly, i.e. read in, unmarshal
// into the minimally required structure and then locate the chart and its urls.
//
// The advantage of this hack: We get a fetchable url we can feed into the part invoked when the
// chart was specified as direct url. No going through a temp file.
func chartArchiveURL(
	c *models.AppChartFull,
	restConfig *restclient.Config,
) (string, error) {
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

	client, err := helm.GetHelmClient(restConfig, "")
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
	content, err := os.ReadFile("/tmp/.helmcache/" + name + "-index.yaml") // nolint:gosec // fixed prefix, name from app chart lookup
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
