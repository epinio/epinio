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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
)

type ListenAddress struct {
	Address     string
	Protocol    string
	FailureMode string
}

type ForwardedPort struct {
	Local  uint16
	Remote uint16
}

func ParseAddresses(addressesToParse []string) ([]ListenAddress, error) {
	var addresses []ListenAddress
	parsed := make(map[string]ListenAddress)
	for _, address := range addressesToParse {
		if address == "localhost" {
			if _, exists := parsed["127.0.0.1"]; !exists {
				ip := ListenAddress{Address: "127.0.0.1", Protocol: "tcp4", FailureMode: "all"}
				parsed[ip.Address] = ip
			}
			if _, exists := parsed["::1"]; !exists {
				ip := ListenAddress{Address: "::1", Protocol: "tcp6", FailureMode: "all"}
				parsed[ip.Address] = ip
			}
		} else if net.ParseIP(address).To4() != nil {
			parsed[address] = ListenAddress{Address: address, Protocol: "tcp4", FailureMode: "any"}
		} else if net.ParseIP(address) != nil {
			parsed[address] = ListenAddress{Address: address, Protocol: "tcp6", FailureMode: "any"}
		} else {
			return nil, fmt.Errorf("%s is not a valid IP", address)
		}
	}
	addresses = make([]ListenAddress, len(parsed))
	id := 0
	for _, v := range parsed {
		addresses[id] = v
		id++
	}
	// Sort addresses before returning to get a stable order
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].Address < addresses[j].Address })

	return addresses, nil
}

/*
valid port specifications:

5000
- forwards from localhost:5000 to pod:5000

8888:5000
- forwards from localhost:8888 to pod:5000

0:5000
:5000
  - selects a random available local port,
    forwards from localhost:<random port> to pod:5000
*/
func ParsePorts(ports []string) ([]ForwardedPort, error) {
	var forwards []ForwardedPort
	for _, portString := range ports {
		parts := strings.Split(portString, ":")
		var localString, remoteString string
		if len(parts) == 1 {
			localString = parts[0]
			remoteString = parts[0]
		} else if len(parts) == 2 {
			localString = parts[0]
			if localString == "" {
				// support :5000
				localString = "0"
			}
			remoteString = parts[1]
		} else {
			return nil, fmt.Errorf("invalid port format '%s'", portString)
		}

		localPort, err := strconv.ParseUint(localString, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("error parsing local port '%s': %s", localString, err)
		}

		remotePort, err := strconv.ParseUint(remoteString, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("error parsing remote port '%s': %s", remoteString, err)
		}
		if remotePort == 0 {
			return nil, fmt.Errorf("remote port must be > 0")
		}

		forwards = append(forwards, ForwardedPort{uint16(localPort), uint16(remotePort)})
	}

	return forwards, nil
}

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
	Logger       logr.Logger
	DialTimeout  time.Duration
	Address      string
	IncomingConn net.Conn
	OutgoingConn net.Conn
}

func NewTCPProxy(ctx context.Context, IncomingConn net.Conn, address string) (*TCPProxy, error) {
	if !strings.Contains(address, ":") {
		address += ":80"
	}
	return &TCPProxy{
		Logger:       requestctx.Logger(ctx).WithName("PortForward"),
		DialTimeout:  time.Minute,
		Address:      address,
		IncomingConn: IncomingConn,
	}, nil
}

func (p *TCPProxy) Start() error {
	var d net.Dialer
	ctxT, cancel := context.WithTimeout(context.Background(), p.DialTimeout)
	defer cancel()

	var err error
	if p.OutgoingConn, err = d.DialContext(ctxT, "tcp", p.Address); err != nil {
		p.Logger.Error(err, "dialing service")
		return err
	}

	return p.handleConnections()
}

func (p *TCPProxy) handleConnections() error {
	defer p.OutgoingConn.Close()
	localError := make(chan error)
	remoteDone := make(chan struct{})

	go func() {
		// Copy from the remote side to the local port.
		if _, err := io.Copy(p.OutgoingConn, p.IncomingConn); err != nil && !errors.Is(err, net.ErrClosed) {
			p.Logger.Error(err, "error copying from IncomingConn to OutgoingConn")
			localError <- err
		}
		// inform the select below that the remote copy is done
		close(remoteDone)
	}()

	go func() {
		// Copy from the local port to the remote side.
		if _, err := io.Copy(p.IncomingConn, p.OutgoingConn); err != nil && !errors.Is(err, net.ErrClosed) {
			p.Logger.Error(err, "error copying from OutgoingConn to IncomingConn")
			localError <- err
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
		break
	case err := <-localError:
		p.Logger.Error(err, "closed TCPProxy for connection", p.IncomingConn.RemoteAddr().String())
		return err
	}

	p.Logger.Info("closed TCPProxy for connection", p.IncomingConn.RemoteAddr().String())

	return nil
}
