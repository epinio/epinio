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
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	thekubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func (hc Controller) PortForward(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	instanceName := c.Query("instance")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	clientSetHTTP1, err := kubernetes.GetHTTP1Client(ctx)
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

	// app exists but has no workload to connect to
	if app.Workload == nil {
		return apierror.NewAPIError("Cannot connect to application without workload", http.StatusBadRequest)
	}

	// TODO: Find podName from application and params (e.g. instance number etc).
	// The application may have more than one pods.
	podNames, err := application.NewWorkload(cluster, app.Meta, app.Workload.DesiredReplicas).PodNames(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}
	if len(podNames) == 0 {
		return apierror.NewAPIError("couldn't find any Pods to connect to", http.StatusBadRequest)
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
			return apierror.NewAPIError("specified instance doesn't exist", http.StatusBadRequest)
		}
	} else {
		podToConnect = podNames[0]
	}

	forwardRequest(c.Writer, c.Request, podToConnect, namespace, clientSetHTTP1)

	return nil
}

func forwardRequest(rw http.ResponseWriter, req *http.Request, podName, namespace string, client thekubernetes.Interface) {
	// https://github.com/kubernetes/kubectl/blob/2acffc93b61e483bd26020df72b9aef64541bd56/pkg/cmd/portforward/portforward.go#L409
	forwardURL := client.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward").
		URL()

	httpClient := client.CoreV1().RESTClient().(*rest.RESTClient).Client
	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = forwardURL
			req.Host = forwardURL.Host
			// let kube authentication work
			delete(req.Header, "Cookie")
			delete(req.Header, "Authorization")
		},
		Transport:     httpClient.Transport,
		FlushInterval: time.Millisecond * 100,
	}

	p.ServeHTTP(rw, req)
}
