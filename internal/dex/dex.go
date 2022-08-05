package dex

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/dchest/uniuri"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/domain"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	// https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
	OutOfBrowserURN = "urn:ietf:wg:oauth:2.0:oob"
)

var (
	// "openid" is a required scope for OpenID Connect flows.
	// Other scopes, such as "groups" can be requested.
	DefaultScopes = []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email", "groups"}
)

// OIDCProvider wraps an oidc.Provider and its Configuration
type OIDCProvider struct {
	Provider *oidc.Provider
	Config   *oauth2.Config
}

// NewOIDCProvider construct an OIDCProvider fetching its configuration
func NewOIDCProvider(ctx context.Context, issuer, clientID string) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, errors.Wrap(err, "creating the provider")
	}

	config := &oauth2.Config{
		Endpoint:    provider.Endpoint(),
		ClientID:    clientID,
		RedirectURL: OutOfBrowserURN,
		Scopes:      DefaultScopes,
	}

	return &OIDCProvider{
		Provider: provider,
		Config:   config,
	}, nil
}

// AuthCodeURLWithPKCE will return an URL that can be used to obtain an auth code, and a code_verifier string.
// The code_verifier is needed to implement the PKCE auth flow, since this is going to be used by our CLI
// Ref: https://www.oauth.com/oauth2-servers/pkce/
func (pc *OIDCProvider) AuthCodeURLWithPKCE() (string, string) {
	state := uniuri.NewLen(32)
	codeVerifier := NewCodeVerifier()

	authCodeURL := pc.Config.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier.Value),
		oauth2.SetAuthURLParam("code_challenge", codeVerifier.ChallengeS256()),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	return authCodeURL, codeVerifier.Value
}

// ExchangeWithPKCE will exchange the authCode with a token, checking if the codeVerifier is valid
func (pc *OIDCProvider) ExchangeWithPKCE(ctx context.Context, authCode, codeVerifier string) (*oauth2.Token, error) {
	token, err := pc.Config.Exchange(ctx, authCode, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, errors.Wrap(err, "exchanging code for token")
	}
	return token, nil
}

// Verify will verify the token, and it will return an oidc.IDToken
func (pc *OIDCProvider) Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	verifier := pc.Provider.Verifier(&oidc.Config{ClientID: pc.Config.ClientID})

	token, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, errors.Wrap(err, "verifying rawIDToken")
	}
	return token, nil
}

func newVerifier(ctx context.Context, clientID string) (*oidc.IDTokenVerifier, error) {
	// Server should always trust the dex url certificate
	// TODO: Mount the actual certificate and ExtendLocalTrust?

	// Initialize a provider by specifying dex's issuer URL.
	domain, err := domain.MainDomain(ctx)
	if err != nil {
		return nil, err
	}
	provider, err := oidc.NewProvider(ctx, fmt.Sprintf("https://auth.%s", domain))
	if err != nil {
		return nil, errors.Wrap(err, "setting up the provider")
	}

	// Create an ID token parser, but only trust ID tokens issued to "example-app"
	return provider.Verifier(&oidc.Config{ClientID: clientID}), nil
}

// TODO (ec) Keeping this for the TODOs
func Verify(ctx context.Context, token string) (*auth.User, error) {
	// TODO: The token was issued to the cli client
	// How can we trust all our clients? E.g. the epinio-ui
	// TODO: Can anyone with access to dex ask if tokens are valid?
	// We didn't use any credentials in the verify process.
	verifier, err := newVerifier(ctx, "epinio-cli")
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
