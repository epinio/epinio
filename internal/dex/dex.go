package dex

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/domain"
	"github.com/pkg/errors"
)

func provider(ctx context.Context) (*oidc.Provider, error) {
	domain, err := domain.MainDomain(ctx)
	if err != nil {
		return nil, err
	}
	providerURL := fmt.Sprintf("https://dex.%s", domain)

	return oidc.NewProvider(ctx, providerURL)
}

// return an HTTP client which trusts the provided root CAs.
func httpClientForRootCAs(rootCAs string) (*http.Client, error) {
	tlsConfig := tls.Config{RootCAs: x509.NewCertPool()}
	rootCABytes, err := os.ReadFile(rootCAs)
	if err != nil {
		return nil, fmt.Errorf("failed to read root-ca: %v", err)
	}
	if !tlsConfig.RootCAs.AppendCertsFromPEM(rootCABytes) {
		return nil, fmt.Errorf("no certs found in root CA file %q", rootCAs)
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}, nil
}

func NewVerifier(ctx context.Context, clientID string) (*oidc.IDTokenVerifier, error) {
	client, err := httpClientForRootCAs("/dex-tls/ca.crt")
	if err != nil {
		return nil, errors.Wrap(err, "creating a client that trusts dex certificate")
	}

	// Initialize a provider by specifying dex's issuer URL.
	provider, err := provider(oidc.ClientContext(ctx, client))
	if err != nil {
		return nil, errors.Wrap(err, "settings up the provider")
	}

	// Create an ID token parser, but only trust ID tokens issued to "example-app"
	return provider.Verifier(&oidc.Config{ClientID: clientID}), nil
}

func Verify(ctx context.Context, token string) (*auth.User, error) {
	verifier, err := NewVerifier(ctx, "epinio-ui")
	if err != nil {
		return nil, err
	}

	// TODO: Nonce validation? (see the "Verify" method's docs)
	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("could not verify bearer token: %v", err)
	}

	// Extract custom claims.
	var claims struct {
		Email    string   `json:"email"`
		Verified bool     `json:"email_verified"`
		Groups   []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %v", err)
	}
	if !claims.Verified {
		return nil, fmt.Errorf("email (%q) in returned claims was not verified", claims.Email)
	}

	// TODO: Populate more fields?
	return &auth.User{Username: claims.Email}, nil
}
