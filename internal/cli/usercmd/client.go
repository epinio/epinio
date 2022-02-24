// Package usercmd provides Epinio CLI commands for users
package usercmd

import (
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/config"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/pkg/errors"

	"github.com/go-logr/logr"
)

var epinioClientMemo *epinioapi.Client

// EpinioClient provides functionality for talking to a
// Epinio installation on Kubernetes
type EpinioClient struct {
	Config *config.Config
	Log    logr.Logger
	ui     *termui.UI
	API    *epinioapi.Client
}

func New() (*EpinioClient, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, errors.Wrap(err, "error loading config")
	}

	ui := termui.NewUI()
	logger := tracelog.NewLogger().WithName("EpinioClient").V(3)

	apiClient, err := getEpinioAPIClient(logger)
	if err != nil {
		return nil, errors.Wrap(err, "error getting Epinio API client")
	}
	serverURL := apiClient.URL

	log := logger.WithName("New")
	log.Info("Ingress API", "url", serverURL)
	log.Info("Config API", "url", cfg.API)

	epinioClient := &EpinioClient{
		API:    apiClient,
		ui:     ui,
		Config: cfg,
		Log:    logger,
	}
	return epinioClient, nil
}

// ClearMemoization clears the memo, so a new call to getEpinioAPIClient does
// not return a cached value
func ClearMemoization() {
	epinioClientMemo = nil
}

func getEpinioAPIClient(logger logr.Logger) (*epinioapi.Client, error) {
	log := logger.WithName("EpinioApiClient")
	defer func() {
		if epinioClientMemo != nil {
			log.Info("return", "api", epinioClientMemo.URL, "wss", epinioClientMemo.WsURL)
			return
		}
		log.Info("return")
	}()

	// Check for information cached in memory, and return if such is found
	if epinioClientMemo != nil {
		log.Info("cached in memory")
		return epinioClientMemo, nil
	}

	// Check for information cached in the Epinio configuration,
	// and return if such is found. Cache into memory as well.
	log.Info("query configuration")

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if cfg.API != "" && cfg.WSS != "" {
		log.Info("cached in config")

		epinioClient := epinioapi.New(log, cfg.API, cfg.WSS, cfg.User, cfg.Password)
		epinioClientMemo = epinioClient

		return epinioClient, nil
	}

	return nil, errors.New("Epinio no longer queries the cluster, please run epinio config update or ask your operator for help")
}
