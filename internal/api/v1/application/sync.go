package application

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// Sync pipes a tar of changed files or a compiled binary directly into the
// running application pod via server-side exec. The client sends the tar to
// the Epinio API; the server streams it into the pod without requiring kubectl
// on the client machine.
//
// Request: multipart with fields:
//   - file: tar archive
//   - mode: "files" (default) or "binary"
//
// "files" mode: extracts into /workspace/source/app
// (interpreted language hot-reload)
//
// "binary" mode: extracts to /tmp, renames to /epinio-sync/app,
// kills child pid for reload
func Sync(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	appName := c.Param("app")

	// Calculate the mode, might need to add more options in the future.
	mode := c.Request.FormValue("mode")
	if mode == "" {
		mode = "files"
	}
	if mode != "files" && mode != "binary" {
		return apierror.NewBadRequestErrorf(
			"invalid mode %q: must be 'files' or 'binary'",
			mode,
		)
	}

	// Check to make sure we aren't going to overwrite the app source with a
	// invalid tarball.
	file, _, formFileError := c.Request.FormFile("file")
	if formFileError != nil {
		return apierror.
			NewBadRequestError(formFileError.Error()).
			WithDetails("can't read multipart file input")
	}

	defer func() {
		fileCloseError := file.Close()
		if fileCloseError != nil {
			log.Errorw("file failed to close", "error", fileCloseError)
		}
	}()

	tarBytes, fileReadError := io.ReadAll(file)
	if fileReadError != nil {
		return apierror.InternalError(fileReadError, "reading uploaded tar")
	}

	cluster, getClusterError := kubernetes.GetCluster(ctx)
	if getClusterError != nil {
		return apierror.InternalError(
			getClusterError,
			"failed to get access to a kube client",
		)
	}

	podName, containerName, apiError := findReadyPod(
		ctx,
		cluster,
		namespace,
		appName,
	)
	if apiError != nil {
		return apiError
	}

	dest := c.Request.FormValue("dest")
	binaryName := c.Request.FormValue("binary_name")

	var cmd []string
	switch mode {
	case "files":
		filesDest := "/workspace/source/app"
		if dest != "" {
			filesDest = dest
		}
		cmd = []string{"tar", "xf", "-", "-C", filesDest, "--overwrite"}
	case "binary":
		binaryDest := "/epinio-sync/app"
		if dest != "" {
			binaryDest = dest
		}
		if binaryName == "" {
			binaryName = "app"
		}
		cmd = []string{
			"sh", "-c",
			fmt.Sprintf(
				"tar xf - -C /tmp && mv /tmp/%s %s && kill $(cat /epinio-sync/pid) 2>/dev/null || true",
				binaryName, binaryDest,
			),
		}
	}

	execURL := cluster.Kubectl.CoreV1().RESTClient().
		Get().
		Namespace(namespace).
		Resource("pods").
		Name(*podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
			Container: *containerName,
			Command:   cmd,
		}, scheme.ParameterCodec).URL()

	executor, execError := remotecommand.NewWebSocketExecutor(
		cluster.RestConfig,
		"GET",
		execURL.String(),
	)
	if execError != nil {
		return apierror.InternalError(execError, "creating exec executor")
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	streamErr := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  bytes.NewReader(tarBytes),
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	})
	if streamErr != nil {
		return apierror.NewAPIError(
			fmt.Sprintf(
				"exec failed: %s (stderr: %s)",
				streamErr.Error(),
				stderrBuf.String(),
			),
			http.StatusInternalServerError,
		)
	}

	log.Infow("sync complete",
		"namespace", namespace,
		"app", appName,
		"pod", podName,
		"mode", mode,
	)

	response.OK(c)
	return nil
}

// findReadyPod returns the pod name and first container name for a ready pod
// of the app.
func findReadyPod(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace, appName string,
) (*string, *string, apierror.APIErrors) {
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=%s", appName)
	pods, listError := cluster.Kubectl.CoreV1().Pods(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if listError != nil {
		return nil, nil, apierror.InternalError(listError, "listing pods for app")
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if !isPodReady(pod) {
			continue
		}

		containerName := ""
		if len(pod.Spec.Containers) > 0 {
			containerName = pod.Spec.Containers[0].Name
		}
		return &pod.Name, &containerName, nil
	}

	return nil, nil, apierror.NewAPIError(
		fmt.Sprintf("no ready pod found for app %s", appName),
		http.StatusServiceUnavailable,
	)
}

func isPodReady(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Ready {
			return true
		}
	}
	return false
}
