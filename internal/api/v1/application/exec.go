package application

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/namespaces"
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

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := namespaces.Exists(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.NamespaceIsNotKnown(namespace)
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	// TODO: Do we need to cleanup anything?
	// defer func() {
	// 	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	// 	defer cancel()
	// 	_ = client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	// }()

	// TODO: Find podName from application and params (e.g. instance number etc).
	// The application may have more than one pods.
	podName := "sample-77b87b5f5b-swfj9"

	proxyRequest(c.Writer, c.Request, podName, namespace, appName, cluster.Kubectl)

	return nil
}

func proxyRequest(rw http.ResponseWriter, req *http.Request, podName, namespace, container string, client thekubernetes.Interface) {
	attachURL := client.CoreV1().RESTClient().
		Get().
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
			Command:   []string{"/bin/bash"},
		}, scheme.ParameterCodec).URL()

	fmt.Printf("attachURL = %+v\n", attachURL)

	// TODO: Impersonate-* stuff. Remove?
	httpClient := client.CoreV1().RESTClient().(*rest.RESTClient).Client
	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = attachURL
			req.Host = attachURL.Host
			for key := range req.Header {
				if strings.HasPrefix(key, "Impersonate-Extra-") {
					delete(req.Header, key)
				}
			}
			delete(req.Header, "Impersonate-Group")
			delete(req.Header, "Impersonate-User")
			delete(req.Header, "Authorization")
			delete(req.Header, "Cookie")
		},
		Transport:     httpClient.Transport,
		FlushInterval: time.Millisecond * 100,
	}

	p.ServeHTTP(rw, req)
}
