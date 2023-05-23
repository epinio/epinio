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

package service

import (
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/proxy"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{} // use default option

func PortForward(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).WithName("PortForward")
	namespace := c.Param("namespace")
	serviceName := c.Param("service")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	service, apiErr := GetService(ctx, cluster, logger, namespace, serviceName)
	if apiErr != nil {
		return apiErr
	}

	if service == nil {
		return apierror.ServiceIsNotKnown(serviceName)
	}

	w, r := c.Writer, c.Request
	wconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error(err, "failed to upgrade")
		return apierror.InternalError(err)
	}

	defer wconn.Close()
	conn := wconn.UnderlyingConn()
	logger.Info("upgraded connection:", conn.RemoteAddr().String())
	logger.Info(fmt.Sprintf("service routes: %+v\n", service.InternalRoutes))

	stopChan := make(<-chan struct{})
	if tcpProxy, err := proxy.NewTCPProxy(c, conn, service.InternalRoutes[0], stopChan); err != nil {
		logger.Error(err, "")
		return apierror.InternalError(err)
	} else {
		if err := tcpProxy.Start(); err != nil {
			logger.Error(err, "")
			return apierror.InternalError(err)
		}
	}

	return nil
}
