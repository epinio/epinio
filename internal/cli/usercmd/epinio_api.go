package usercmd

import (
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/config"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/pkg/errors"
)

var epinioClientMemo *epinioapi.Client

func getEpinioAPIClient() (*epinioapi.Client, error) {
	log := tracelog.NewLogger().WithName("EpinioApiClient").V(3)
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

// ClearMemoization clears the memo, so a new call to getEpinioAPIClient does
// not return a cached value
func ClearMemoization() {
	epinioClientMemo = nil
}
