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
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/proxy"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var appPortForwardUpgrader = websocket.Upgrader{} // use default options

func PortForward(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	instanceName := c.Query("instance")

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

	pod, err := cluster.Kubectl.CoreV1().Pods(namespace).Get(ctx, podToConnect, metav1.GetOptions{})
	if err != nil {
		return apierror.InternalError(err)
	}
	if pod.Status.PodIP == "" {
		return apierror.NewAPIError("selected instance does not have a pod IP yet", http.StatusBadRequest)
	}

	remotePort := int32(8080)
	for _, container := range pod.Spec.Containers {
		if len(container.Ports) > 0 {
			remotePort = container.Ports[0].ContainerPort
			break
		}
	}

	wconn, err := appPortForwardUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return apierror.InternalError(err)
	}
	defer wconn.Close()

	target := fmt.Sprintf("%s:%d", pod.Status.PodIP, remotePort)
	tcpProxy, err := proxy.NewTCPProxy(c, wconn.UnderlyingConn(), target)
	if err != nil {
		return apierror.InternalError(err)
	}
	if err := tcpProxy.Start(); err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
