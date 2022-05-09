package application

import (
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	thekubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

func (hc Controller) Exec(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	instanceName := c.Query("instance")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	if err := hc.validateNamespace(ctx, cluster, namespace); err != nil {
		return err
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	// app exists but has no workload to connect to
	if app.Workload == nil {
		return apierror.NewAPIError("Cannot connect to application without workload",
			"", http.StatusBadRequest)
	}

	workload := application.NewWorkload(cluster, app.Meta)
	podNames, err := workload.PodNames(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	if len(podNames) < 1 {
		return apierror.NewAPIError("couldn't find any Instances to connect to",
			"", http.StatusBadRequest)
	}

	podToConnect := ""
	if instanceName != "" {
		for _, podName := range podNames {
			if podName == instanceName {
				podToConnect = podName
				break
			}
		}

		if podToConnect == "" {
			return apierror.NewAPIError("specified instance doesn't exist",
				"", http.StatusBadRequest)
		}
	} else {
		podToConnect = podNames[0]
	}

	deployment, err := workload.Deployment(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	proxyRequest(c.Writer, c.Request, podToConnect, namespace, deployment.Name, cluster.Kubectl)

	return nil
}

func proxyRequest(rw http.ResponseWriter, req *http.Request, podName, namespace, container string, client thekubernetes.Interface) {
	// https://github.com/kubernetes/kubectl/blob/2acffc93b61e483bd26020df72b9aef64541bd56/pkg/cmd/exec/exec.go#L352
	attachURL := client.CoreV1().RESTClient().
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
			Container: container,
			// https://github.com/rancher/dashboard/blob/37f40d7213ff32096bfefd02de77be6a0e7f40ab/components/nav/WindowManager/ContainerShell.vue#L22
			Command: []string{"/bin/sh", "-c", "TERM=xterm-256color; export TERM; exec /bin/bash"},
		}, scheme.ParameterCodec).URL()

	httpClient := client.CoreV1().RESTClient().(*rest.RESTClient).Client
	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = attachURL
			req.Host = attachURL.Host
			// let kube authentication work
			delete(req.Header, "Cookie")
			delete(req.Header, "Authorization")
		},
		Transport:     httpClient.Transport,
		FlushInterval: time.Millisecond * 100,
	}

	p.ServeHTTP(rw, req)
}
