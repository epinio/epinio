package dex

import (
	"context"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/dchest/uniuri"
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
	Issuer   string
	Endpoint *url.URL
	Provider *oidc.Provider
	Config   *oauth2.Config
}

// NewOIDCProvider construct an OIDCProvider fetching its configuration
func NewOIDCProvider(ctx context.Context, issuer, clientID string) (*OIDCProvider, error) {
	endpoint, err := url.Parse(issuer)
	if err != nil {
		return nil, errors.Wrap(err, "parsing the issuer URL")
	}

	return NewOIDCProviderWithEndpoint(ctx, issuer, clientID, endpoint)
}

// NewOIDCProviderWithEndpoint construct an OIDCProvider fetching its configuration from the endpoint URL
func NewOIDCProviderWithEndpoint(ctx context.Context, issuer, clientID string, endpoint *url.URL) (*OIDCProvider, error) {
	// If the issuer is different from the endpoint we need to set it in the context.
	// With this differentiation the Epinio server can reach the Dex service through the Kubernetes DNS
	// instead of the external URL. This was causing issues when the host was going to be resolved as a local IP (i.e: Rancher Desktop).
	// - https://github.com/epinio/epinio/issues/1781
	if issuer != endpoint.String() {
		ctx = oidc.InsecureIssuerURLContext(ctx, issuer)
	}

	provider, err := oidc.NewProvider(ctx, endpoint.String())
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
		Issuer:   issuer,
		Endpoint: endpoint,
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

// AddScopes will add scopes to the OIDCProvider.Config.Scopes, extending the DefaultScopes
func (pc *OIDCProvider) AddScopes(scopes ...string) {
	pc.Config.Scopes = append(pc.Config.Scopes, scopes...)
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
	keySet := oidc.NewRemoteKeySet(ctx, pc.Endpoint.String()+"/keys")
	verifier := oidc.NewVerifier(pc.Issuer, keySet, &oidc.Config{ClientID: pc.Config.ClientID})

	token, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, errors.Wrap(err, "verifying rawIDToken")
	}
	return token, nil
}
