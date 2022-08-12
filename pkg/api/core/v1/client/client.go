// Package client connects to the Epinio API's endpoints
package client

import (
	"net/http"

	"context"
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
	log        logr.Logger
	Settings   *epiniosettings.Settings
	HttpClient *http.Client
}

// New returns a new Epinio API client
func New(settings *epiniosettings.Settings) *Client {
	log := tracelog.NewLogger().WithName("EpinioApiClient").V(3)
	ctx := context.Background()

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
			tokenSource = oidcProvider.Config.TokenSource(ctx, token)
		}
	}

	if settings.Certs != "" {
		auth.ExtendLocalTrust(settings.Certs)
	}

	return &Client{
		log:        log,
		Settings:   settings,
		HttpClient: oauth2.NewClient(ctx, tokenSource),
	}
}
