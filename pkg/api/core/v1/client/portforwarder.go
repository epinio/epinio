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
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
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
	parsedAddresses, err := parseAddresses(addresses)
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, errors.New("you must specify at least 1 port")
	}
	parsedPorts, err := parsePorts(ports)
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

func parseAddresses(addressesToParse []string) ([]ListenAddress, error) {
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
func parsePorts(ports []string) ([]ForwardedPort, error) {
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

func (pf *ServicePortForwarder) ForwardPorts() error {
	return pf.forward()
}

func (pf *ServicePortForwarder) forward() error {
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
	go pf.waitForConnection(listener, *port)
	return nil
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

func (pf *ServicePortForwarder) waitForConnection(listener net.Listener, port ForwardedPort) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "use of closed network connection") {
				pf.Client.log.V(1).Info("error accepting connection on port %d: %v", port.Local, err)
			}
			return
		}
		go pf.handleConnection(conn)
	}
}

func (pf *ServicePortForwarder) handleConnection(localConn net.Conn) {
	defer localConn.Close()

	portForwardURL, err := url.Parse(pf.Endpoint)
	if err != nil {
		pf.Client.log.V(1).Error(err, "error parsing endpoint")
		return
	}

	if err := pf.Client.addAuthTokenToURL(portForwardURL); err != nil {
		pf.Client.log.V(1).Error(err, "error adding auth token to URL")
		return
	}

	portForwardURL.Scheme = "wss"

	c, _, err := websocket.DefaultDialer.Dial(portForwardURL.String(), nil)
	if err != nil {
		pf.Client.log.V(1).Error(err, "error dialing")
		return
	}
	if c != nil {
		defer c.Close()
	}

	upgradedConnection := c.UnderlyingConn()
	defer upgradedConnection.Close()

	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	go func() {
		// Copy from the remote side to the local port.
		if _, err := io.Copy(localConn, upgradedConnection); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			pf.Client.log.V(1).Info("error copying from upgradedConnection to local connection: %v", err)
		}

		// inform the select below that the remote copy is done
		close(remoteDone)
	}()

	go func() {
		// Copy from the local port to the remote side.
		if _, err := io.Copy(upgradedConnection, localConn); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			pf.Client.log.V(1).Info("error copying from local connection to UpgradedConnection: %v", err)
			// break out of the select below without waiting for the other copy to finish
			close(localError)
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}
}
