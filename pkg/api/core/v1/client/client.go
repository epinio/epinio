// Package client connects to the Epinio API's endpoints
package client

import (
	"net/http"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/auth"
	epiniosettings "github.com/epinio/epinio/internal/cli/settings"
	"github.com/go-logr/logr"
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

	if settings.Certs != "" {
		auth.ExtendLocalTrust(settings.Certs)
	}

	return &Client{
		log:        log,
		Settings:   settings,
		HttpClient: http.DefaultClient,
	}
}
