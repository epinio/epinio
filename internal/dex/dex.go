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

package dex

import (
	"context"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/dchest/uniuri"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"golang.org/x/oauth2"
)

const (
	// https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
	OutOfBrowserURN = "urn:ietf:wg:oauth:2.0:oob"
)

var (
	// "openid" is a required scope for OpenID Connect flows.
	// Other scopes, such as "groups" can be requested.
	DefaultScopes = []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email", "groups", "federated:id"}
)

// OIDCProvider wraps an oidc.Provider and its Configuration
type OIDCProvider struct {
	Config Config

	Provider *oidc.Provider
}

// NewOIDCProvider construct an OIDCProvider loading the configuration from the issuer URL
func NewOIDCProvider(ctx context.Context, issuer, clientID string) (*OIDCProvider, error) {
	config, err := NewConfig(issuer, clientID)
	if err != nil {
		return nil, errors.Wrap(err, "creating dex configuration")
	}

	return NewOIDCProviderWithConfig(ctx, config)
}

// NewOIDCProviderWithConfig construct an OIDCProvider with the provided configuration
func NewOIDCProviderWithConfig(ctx context.Context, config Config) (*OIDCProvider, error) {
	// If the issuer is different from the endpoint we need to set it in the context.
	// With this differentiation the Epinio server can reach the Dex service through the Kubernetes DNS
	// instead of the external URL. This was causing issues when the host was going to be resolved as a local IP (i.e: Rancher Desktop).
	// - https://github.com/epinio/epinio/issues/1781
	if config.Issuer != config.Endpoint.String() && strings.HasSuffix(config.Endpoint.Hostname(), ".svc.cluster.local") {
		ctx = oidc.InsecureIssuerURLContext(ctx, config.Issuer)
	}

	provider, err := oidc.NewProvider(ctx, config.Endpoint.String())
	if err != nil {
		return nil, errors.Wrap(err, "creating the provider")
	}

	config.Oauth2 = &oauth2.Config{
		Endpoint:    provider.Endpoint(),
		ClientID:    config.ClientID,
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

	authCodeURL := pc.Config.Oauth2.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier.Value),
		oauth2.SetAuthURLParam("code_challenge", codeVerifier.ChallengeS256()),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	return authCodeURL, codeVerifier.Value
}

// AddScopes will add scopes to the OIDCProvider.Config.Scopes, extending the DefaultScopes
func (pc *OIDCProvider) AddScopes(scopes ...string) {
	pc.Config.Oauth2.Scopes = append(pc.Config.Oauth2.Scopes, scopes...)
}

// ExchangeWithPKCE will exchange the authCode with a token, checking if the codeVerifier is valid
func (pc *OIDCProvider) ExchangeWithPKCE(ctx context.Context, authCode, codeVerifier string) (*oauth2.Token, error) {
	token, err := pc.Config.Oauth2.Exchange(ctx, authCode, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, errors.Wrap(err, "exchanging code for token")
	}
	return token, nil
}

// Verify will verify the token, and it will return an oidc.IDToken
func (pc *OIDCProvider) Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	keySet := oidc.NewRemoteKeySet(ctx, pc.Config.Endpoint.String()+"/keys")
	verifier := oidc.NewVerifier(pc.Config.Issuer, keySet, &oidc.Config{ClientID: pc.Config.Oauth2.ClientID})

	token, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, errors.Wrap(err, "verifying rawIDToken")
	}
	return token, nil
}

// GetProviderGroups returns the ProviderGroups of the specified provider
func (pc *OIDCProvider) GetProviderGroups(providerID string) (*ProviderGroups, error) {
	for _, pg := range pc.Config.ProvidersGroups {
		if pg.ConnectorID == providerID {
			return &pg, nil
		}
	}

	return nil, errors.Errorf("provider '%s' not found", providerID)
}

// GetRoleFromGroups returns the roles matching the provided groups
func (pg *ProviderGroups) GetRolesFromGroups(groupIDs ...string) []string {
	roles := []string{}

	for _, g := range pg.Groups {
		if slices.Contains(groupIDs, g.ID) {
			roles = append(roles, g.Roles...)
		}
	}

	return roles
}
