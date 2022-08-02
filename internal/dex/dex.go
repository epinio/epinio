package dex

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/domain"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// AuthURL returns a URL the user should visit to authenticate with dex
func AuthURL(ctx context.Context, providerURL, clientID, clientSecret string, scopes []string) (string, error) {
	// TODO: Remove the volume mounted in the helm chart (not needed any more)
	// We will do this trick in the web UI Pod instead.
	// httpClient, err := httpClientForRootCAs([]byte(epinioCert))
	// if err != nil {
	// 	return "", errors.Wrap(err, "creating a client that trusts dex certificate")
	// }
	httpClient := http.DefaultClient

	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, httpClient), providerURL)
	if err != nil {
		return "", errors.Wrap(err, "creating the provider")
	}

	oauth2Config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		// https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
		RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
		Endpoint:    provider.Endpoint(),
		// "openid" is a required scope for OpenID Connect flows.
		// Other scopes, such as "groups" can be requested.
		Scopes: []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}

	// What scopes does a provider support?
	// See: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
	var s struct {
		ScopesSupported []string `json:"scopes_supported"`
	}

	err = provider.Claims(&s)
	if err != nil {
		return "", errors.Wrap(err, "finding supported claims")
	}

	var offlineAsScope bool
	if len(s.ScopesSupported) == 0 {
		// scopes_supported is a "RECOMMENDED" discovery claim, not a required
		// one. If missing, assume that the provider follows the spec and has
		// an "offline_access" scope.
		offlineAsScope = true
	} else {
		// See if scopes_supported has the "offline_access" scope.
		offlineAsScope = func() bool {
			for _, scope := range s.ScopesSupported {
				if scope == oidc.ScopeOfflineAccess {
					return true
				}
			}
			return false
		}()
	}

	authCodeURL := ""
	oauth2Config.Scopes = append(oauth2Config.Scopes, "openid", "profile", "email")
	// TODO: Should we allow the user to choose whether offline_access is wanted or not?
	if offlineAsScope {
		// TODO: Otherwise, will refresh tokens work? How? (Navigate to AccessTypeOffline below)
		oauth2Config.Scopes = append(oauth2Config.Scopes, "offline_access")
		authCodeURL = oauth2Config.AuthCodeURL(AppState, oauth2.AccessTypeOffline)
	} else {
		authCodeURL = oauth2Config.AuthCodeURL(AppState)
	}

	return authCodeURL, nil
}

// return an HTTP client which trusts the provided root CAs.
func httpClientForRootCAs(rootCAs []byte) (*http.Client, error) {
	tlsConfig := tls.Config{RootCAs: x509.NewCertPool()}
	if !tlsConfig.RootCAs.AppendCertsFromPEM(rootCAs) {
		return nil, fmt.Errorf("no certs found in root CA file %q", rootCAs)
	}
	return &http.Client{
		Transport: http.DefaultTransport,
		// Transport: &http.Transport{
		// 	TLSClientConfig: &tlsConfig,
		// 	Proxy:           http.ProxyFromEnvironment,
		// 	Dial: (&net.Dialer{
		// 		Timeout:   30 * time.Second,
		// 		KeepAlive: 30 * time.Second,
		// 	}).Dial,
		// 	TLSHandshakeTimeout:   10 * time.Second,
		// 	ExpectContinueTimeout: 1 * time.Second,
		//},
	}, nil
}

func NewVerifier(ctx context.Context, clientID string) (*oidc.IDTokenVerifier, error) {
	client := http.DefaultClient // httpClientForRootCAs([]byte(epinioCert))
	// if err != nil {
	// 	return nil, errors.Wrap(err, "creating a client that trusts dex certificate")
	// }

	// Initialize a provider by specifying dex's issuer URL.
	domain, err := domain.MainDomain(ctx)
	if err != nil {
		return nil, err
	}
	provider, err := oidc.NewProvider(
		oidc.ClientContext(ctx, client),
		fmt.Sprintf("https://dex.%s", domain))
	if err != nil {
		return nil, errors.Wrap(err, "setting up the provider")
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
