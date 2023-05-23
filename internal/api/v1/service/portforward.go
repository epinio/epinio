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
	"strings"

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

	wconn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err, "failed to upgrade")
		return apierror.InternalError(err)
	}
	defer wconn.Close()

	conn := wconn.UnderlyingConn()

	msg := fmt.Sprintf("upgraded connection [%s] - service routes [%s]", conn.RemoteAddr().String(), strings.Join(service.InternalRoutes, ", "))
	logger.V(1).Info(msg)

	if len(service.InternalRoutes) == 0 {
		return apierror.NewInternalError("no internal service routes available")
	}

	tcpProxy, err := proxy.NewTCPProxy(c, conn, service.InternalRoutes[0])
	if err != nil {
		return apierror.InternalError(err)
	}
	if err := tcpProxy.Start(); err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
