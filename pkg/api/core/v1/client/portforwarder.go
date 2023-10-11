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

package client

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"

	. "github.com/epinio/epinio/internal/api/v1/proxy"
	"github.com/gorilla/websocket"
)

type ServicePortForwarder struct {
	Client             *Client
	Endpoint           string
	ListeningAddresses []ListenAddress
	Ports              []ForwardedPort
	StopChan           <-chan struct{}
}

func NewServicePortForwarder(client *Client, endpoint string, addresses []string, ports []string, stopChan <-chan struct{}) (*ServicePortForwarder, error) {
	if len(addresses) == 0 {
		return nil, errors.New("you must specify at least 1 address")
	}
	parsedAddresses, err := ParseAddresses(addresses)
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, errors.New("you must specify at least 1 port")
	}
	parsedPorts, err := ParsePorts(ports)
	if err != nil {
		return nil, err
	}
	return &ServicePortForwarder{
		Client:             client,
		Endpoint:           endpoint,
		ListeningAddresses: parsedAddresses,
		Ports:              parsedPorts,
		StopChan:           stopChan,
	}, nil
}

func (pf *ServicePortForwarder) ForwardPorts() error {
	var err error

	listenSuccess := false
	for i := range pf.Ports {
		port := &pf.Ports[i]
		err = pf.listenOnPort(port)
		switch {
		case err == nil:
			listenSuccess = true
		default:
			pf.Client.log.V(1).Info("Unable to listen on port %d: %v\n", port.Local)
			return err
		}
	}

	if !listenSuccess {
		return fmt.Errorf("unable to listen on any of the requested ports: %v", pf.Ports)
	}

	// wait for interrupt or conn closure
	<-pf.StopChan
	pf.Client.log.V(1).Info("lost connection to pod")

	return nil
}

func (pf *ServicePortForwarder) listenOnPort(port *ForwardedPort) error {
	var errors []error
	failCounters := make(map[string]int, 2)
	successCounters := make(map[string]int, 2)
	for _, addr := range pf.ListeningAddresses {
		err := pf.listenOnPortAndAddress(port, addr.Protocol, addr.Address)
		if err != nil {
			errors = append(errors, err)
			failCounters[addr.FailureMode]++
		} else {
			successCounters[addr.FailureMode]++
		}
	}
	if successCounters["all"] == 0 && failCounters["all"] > 0 {
		return fmt.Errorf("%s: %v", "Listeners failed to create with the following errors", errors)
	}
	if failCounters["any"] > 0 {
		return fmt.Errorf("%s: %v", "Listeners failed to create with the following errors", errors)
	}
	return nil
}

func (pf *ServicePortForwarder) listenOnPortAndAddress(port *ForwardedPort, protocol string, address string) error {
	listener, err := pf.getListener(protocol, address, port)
	if err != nil {
		return err
	}
	go func() {
		if err = pf.waitForConnection(listener, *port); err != nil {
			return
		}
	}()
	return err
}

func (pf *ServicePortForwarder) getListener(protocol string, hostname string, port *ForwardedPort) (net.Listener, error) {
	listener, err := net.Listen(protocol, net.JoinHostPort(hostname, strconv.Itoa(int(port.Local))))
	if err != nil {
		return nil, fmt.Errorf("unable to create listener: Error %s", err)
	}
	listenerAddress := listener.Addr().String()
	host, localPort, _ := net.SplitHostPort(listenerAddress)
	localPortUInt, err := strconv.ParseUint(localPort, 10, 16)

	if err != nil {
		errStr := fmt.Sprintf("error parsing local port: %s from %s (%s)", err, listenerAddress, host)
		return nil, fmt.Errorf(errStr)
	}
	port.Local = uint16(localPortUInt)
	pf.Client.log.V(1).Info("Forwarding from %s -> %d\n", net.JoinHostPort(hostname, strconv.Itoa(int(localPortUInt))), port.Remote)

	return listener, nil
}

func (pf *ServicePortForwarder) waitForConnection(listener net.Listener, port ForwardedPort) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			pf.Client.log.V(1).Info("error accepting connection on port %d: %v", port.Local, err)
			return err
		} else {
			defer listener.Close()
		}

		go func() {
			if err = pf.handleConnection(conn); err != nil {
				return
			}
		}()
	}
}

func (pf *ServicePortForwarder) handleConnection(localConn net.Conn) error {
	defer localConn.Close()

	portForwardURL, err := url.Parse(pf.Endpoint)
	if err != nil {
		pf.Client.log.V(1).Error(err, "error parsing endpoint")
		return err
	}

	if err := pf.Client.addAuthTokenToURL(portForwardURL); err != nil {
		pf.Client.log.V(1).Error(err, "error adding auth token to URL")
		return err
	}

	portForwardURL.Scheme = "wss"

	c, _, err := websocket.DefaultDialer.Dial(portForwardURL.String(), pf.Client.Headers())
	if err != nil {
		pf.Client.log.V(1).Error(err, "error dialing")
		return err
	}
	if c != nil {
		defer c.Close()
	}

	upgradedConnection := c.UnderlyingConn()
	defer upgradedConnection.Close()

	localError := make(chan error)
	remoteDone := make(chan struct{})

	go func() {
		// Copy from the remote side to the local port.
		if _, err := io.Copy(localConn, upgradedConnection); err != nil && !errors.Is(err, net.ErrClosed) {
			pf.Client.log.V(1).Error(err, "error copying from upgradedConnection to local connection")
			localError <- err
		}

		// inform the select below that the remote copy is done
		close(remoteDone)
	}()

	go func() {
		// Copy from the local port to the remote side.
		if _, err := io.Copy(upgradedConnection, localConn); err != nil && !errors.Is(err, net.ErrClosed) {
			pf.Client.log.V(1).Error(err, "error copying from local connection to UpgradedConnection")
			// break out of the select below without waiting for the other copy to finish
			localError <- err
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
		break
	case err := <-localError:
		pf.Client.log.Error(err, "closed ServicePortForwarder for connection", upgradedConnection.RemoteAddr().String())
		return err
	}

	return nil
}
