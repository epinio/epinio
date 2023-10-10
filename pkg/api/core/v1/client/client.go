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

// Package client connects to the Epinio API's endpoints
package client

import (
	"context"
	"net/http"
	"regexp"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/auth"
	epiniosettings "github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/dex"
	"github.com/go-logr/logr"
	"golang.org/x/oauth2"
)

// Client provides functionality for talking to an Epinio API
// server
type Client struct {
	log              logr.Logger
	Settings         *epiniosettings.Settings
	HttpClient       *http.Client
	customHeaders    http.Header
	noVersionWarning bool
}

// New returns a new Epinio API client
func New(ctx context.Context, settings *epiniosettings.Settings) *Client {
	log := tracelog.NewLogger().WithName("EpinioApiClient").V(3)

	if settings.Certs != "" {
		auth.ExtendLocalTrust(settings.Certs)
	}

	var tokenSource oauth2.TokenSource

	// we have to initialize the tokenSource (for the refresh) only if there is already a token to refresh
	// otherwise we could hit an untrusted CA
	if settings.API != "" && settings.Token.AccessToken != "" {
		dexURL := regexp.MustCompile(`epinio\.(.*)`).ReplaceAllString(settings.API, "auth.$1")
		token := &oauth2.Token{
			AccessToken:  settings.Token.AccessToken,
			RefreshToken: settings.Token.RefreshToken,
			Expiry:       settings.Token.Expiry,
			TokenType:    settings.Token.TokenType,
		}

		oidcProvider, err := dex.NewOIDCProvider(ctx, dexURL, "epinio-cli")
		if err != nil {
			log.Info("error creating the OIDC provider", "error", err.Error())
		} else {
			// ask a token for the 'epinio-api' client
			oidcProvider.AddScopes("audience:server:client_id:epinio-api")
			tokenSource = oidcProvider.Config.Oauth2.TokenSource(ctx, token)
		}
	}

	if settings.Certs != "" {
		auth.ExtendLocalTrust(settings.Certs)
	}

	return &Client{
		log:           log,
		Settings:      settings,
		HttpClient:    oauth2.NewClient(ctx, tokenSource),
		customHeaders: http.Header{},
	}
}

func (c *Client) SetHeader(key, value string) {
	if c.customHeaders != nil {
		c.customHeaders.Set(key, value)
	}
}

func (c *Client) Headers() http.Header {
	return c.customHeaders
}
