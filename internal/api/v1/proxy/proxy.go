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

package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/rest"
)

func RunProxy(ctx context.Context, rw http.ResponseWriter, req *http.Request, destination *url.URL) apierror.APIErrors {
	clientSetHTTP1, err := kubernetes.GetHTTP1Client(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	httpClient := clientSetHTTP1.CoreV1().RESTClient().(*rest.RESTClient).Client

	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = destination
			req.Host = destination.Host
			// let kube authentication work
			delete(req.Header, "Cookie")
			delete(req.Header, "Authorization")
		},
		Transport:     httpClient.Transport,
		FlushInterval: time.Millisecond * 100,
	}

	p.ServeHTTP(rw, req)

	return nil
}

type TCPProxy struct {
	GCtx               *gin.Context
	Address            string
	StopChan           <-chan struct{}
	UpgradedConnection net.Conn
	ProxyConnection    net.Conn
}

func NewTCPProxy(gctx *gin.Context, upgradedConnection net.Conn, address string, stopChan <-chan struct{}) (*TCPProxy, error) {
	if !strings.Contains(address, ":") {
		address += ":80"
	}
	return &TCPProxy{
		GCtx:               gctx,
		UpgradedConnection: upgradedConnection,
		Address:            address,
		StopChan:           stopChan,
	}, nil
}

func (p *TCPProxy) Start() error {
	ctx := p.GCtx.Request.Context()
	logger := requestctx.Logger(ctx).WithName("PortForward")
	var d net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var err error
	if p.ProxyConnection, err = d.DialContext(ctx, "tcp", p.Address); err != nil {
		logger.Error(err, "dialing service")
		return err
	}

	return p.handleConnections()
}

func (p *TCPProxy) handleConnections() error {
	defer p.ProxyConnection.Close()
	ctx := p.GCtx.Request.Context()
	logger := requestctx.Logger(ctx).WithName("PortForward")
	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	go func() {
		// Copy from the remote side to the local port.
		if _, err := io.Copy(p.ProxyConnection, p.UpgradedConnection); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			logger.Error(err, "error copying from UpgradedConnection to ProxyConnection")
			close(localError)
		}
		// inform the select below that the remote copy is done
		close(remoteDone)
	}()

	go func() {
		// Copy from the local port to the remote side.
		if _, err := io.Copy(p.UpgradedConnection, p.ProxyConnection); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			logger.Error(err, "error copying from ProxyConnection to UpgradedConnection")
			close(localError)
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}

	logger.Info("closed TCPProxy for connection", p.UpgradedConnection.RemoteAddr().String())

	return nil
}
