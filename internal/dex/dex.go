package dex

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/domain"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// Oauth2Config returns an Oauth2Config
func Oauth2Config(ctx context.Context, providerURL, clientID, clientSecret string) (*oauth2.Config, error) {
	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		return nil, errors.Wrap(err, "creating the provider")
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		// https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
		RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
		Endpoint:    provider.Endpoint(),
		// "openid" is a required scope for OpenID Connect flows.
		// Other scopes, such as "groups" can be requested.
		Scopes: []string{oidc.ScopeOpenID, "profile", "email", "groups", "offline_access"},
	}, nil
}

func NewVerifier(ctx context.Context, clientID string) (*oidc.IDTokenVerifier, error) {
	// Server should always trust the dex url certificate
	// TODO: Mount the actual certificate and ExtendLocalTrust?
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

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
	// TODO: The token was issued to the cli client
	// How can we trust all our clients? E.g. the epinio-ui
	// TODO: Can anyone with access to dex ask if tokens are valid?
	// We didn't use any credentials in the verify process.
	verifier, err := NewVerifier(ctx, "epinio-cli")
	if err != nil {
		return nil, errors.Wrap(err, "setting up the verifier")
	}

	// TODO: Nonce validation? (see the "Verify" method's docs)
	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, errors.Wrap(err, "verifing bearer token")
	}

	// Extract custom claims.
	var claims struct {
		Email    string   `json:"email"`
		Verified bool     `json:"email_verified"`
		Groups   []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, errors.Wrap(err, "parsing claims")
	}
	// TODO: How should they verify?
	// if !claims.Verified {
	// 	return nil, errors.Wrapf(err, "email (%q) in returned claims was not verified", claims.Email)
	// }

	// TODO: Populate more fields?
	// TODO: Set role based on existing user in Kubernetes secret
	// TODO: Don't hardcode "admin" here!!!!!!
	return &auth.User{Username: claims.Email, Role: "admin"}, nil
}
