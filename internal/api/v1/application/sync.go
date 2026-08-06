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

	dest := c.Request.FormValue("dest")
	mode := c.Request.FormValue("mode")
	binaryName := c.Request.FormValue("binary_name")

	// Calculate the mode, might need to add more options in the future.
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

	//Ensure the pod is ready before trying to exec int it.
	podName, containerName, apiError := findReadyPod(
		ctx,
		cluster,
		namespace,
		appName,
	)
	if apiError != nil {
		return apiError
	}

	cmd := buildSyncCommand(mode, dest, binaryName)

	// Set up the exec request to stream the tar into the pod.
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

	// Actually execute the command in the pod, streaming the tar as stdin and
	// capturing stdout/stderr for logging and error handling.
	executor, execError := remotecommand.NewWebSocketExecutor(
		cluster.RestConfig,
		"GET",
		execURL.String(),
	)
	if execError != nil {
		return apierror.InternalError(execError, "creating exec executor")
	}

	// Capture any errors from the exce command
	var stdoutBuf, stderrBuf bytes.Buffer
	streamError := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  bytes.NewReader(tarBytes),
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
	})
	if streamError != nil {
		return apierror.NewAPIError(
			fmt.Sprintf(
				"exec failed: %s (stderr: %s)",
				streamError.Error(),
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

// buildSyncCommand returns the in-pod command for the given sync mode. For
// "files" mode the tar is extracted directly into the app source directory.
// For "binary" mode the tar is extracted to a temp location and the binary is
// moved to the sync directory. Both modes kill the app process afterwards so
// the supervisor restarts it with the new content. The "dest" value overrides
// the default destination for flexibility of app deployment structures.
//
// The kill targets a process GROUP (note the leading "-" on the pid): the
// supervisor launches the app via setsid, so the recorded pid is the group
// leader. Killing the group reaps entrypoints that fork or background the
// real server (e.g. a start.sh doing `( node app ) &`); killing only the
// recorded pid would leave the real server orphaned and still holding its
// port, so the relaunched process could not bind.
//
// Positional args ($1, $2) are used so paths with spaces or special
// characters are handled correctly without string interpolation. kill is
// intentionally allowed to fail (pid file may not exist yet).
func buildSyncCommand(mode, dest, binaryName string) []string {
	switch mode {
	case "files":
		filesDest := "/workspace/source/app"
		if dest != "" {
			filesDest = dest
		}
		return []string{
			"sh", "-c",
			`chmod -R u+w "$1" && tar xf - -C "$1" --overwrite && { kill -9 -"$(cat /epinio-sync/pid)" 2>/dev/null; true; }`,
			"--", filesDest,
		}
	case "binary":
		binaryDest := "/epinio-sync/app"
		if dest != "" {
			binaryDest = dest
		}
		if binaryName == "" {
			binaryName = "app"
		}
		return []string{
			"sh", "-c",
			`tar xf - -C /tmp && mv /tmp/"$1" "$2" && { kill -9 -"$(cat /epinio-sync/pid)" 2>/dev/null; true; }`,
			"--", binaryName, binaryDest,
		}
	}
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
