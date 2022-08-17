package auth

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

// ExtendLocalTrust makes the certs found in specified PEM string
// available as root CA certs, beyond the standard certs. It does this
// by creating an in-memory pool of certs filled from both the system
// pool and the argument, and setting this as the cert origin for
// net/http's default transport. Ditto for the websocket's default
// dialer.
func ExtendLocalTrust(certs string) {
	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	rootCAs.AppendCertsFromPEM([]byte(certs))

	// Trust the augmented cert pool in our client
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = config
	websocket.DefaultDialer.TLSClientConfig = config

	// See https://github.com/gorilla/websocket/issues/601 for
	// what this is a work around for.
	http.DefaultTransport.(*http.Transport).ForceAttemptHTTP2 = false
}

// ExtendLocalTrustFromFile will load a cert from the specified file and will extend the local trust
func ExtendLocalTrustFromFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	ExtendLocalTrust(string(content))
	return nil
}
