package clients

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/config"
	epinioapi "github.com/epinio/epinio/pkg/epinioapi/v1/client"
	"github.com/pkg/errors"
)

const (
	epinioAPIProtocol = "https"
	epinioWSProtocol  = "wss"
)

var epinioClientMemo *epinioapi.Client

func getEpinioAPIClient(ctx context.Context) (*epinioapi.Client, error) {
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

	log.Info("query cluster")

	// Not cached at all. Query and return the cluster
	// ingress. Cache to configuration, and memory.

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	epinioURL, epinioWsURL, err := getEpinioURL(ctx, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve epinio api host")
	}

	log.Info("retrieved from ingress", "api", epinioURL, "wss", epinioWsURL)

	cfg.API = epinioURL
	cfg.WSS = epinioWsURL

	err = cfg.Save()
	if err != nil {
		return nil, errors.Wrap(err, "failed to save configuration")
	}

	epinioClient := epinioapi.New(log, cfg.API, cfg.WSS, cfg.User, cfg.Password)
	epinioClientMemo = epinioClient

	return epinioClient, nil
}

// ClearMemoization clears the memo, so a new call to getEpinioAPIClient does
// not return a cached value
func ClearMemoization() {
	epinioClientMemo = nil
}

// getEpinioURL finds the URL's for epinio from the cluster
func getEpinioURL(ctx context.Context, cluster *kubernetes.Cluster) (string, string, error) {
	// Get the ingress
	ingresses, err := cluster.ListIngress(ctx, deployments.EpinioDeploymentID, "app.kubernetes.io/name=epinio")
	if err != nil {
		return "", "", errors.Wrap(err, "failed to list ingresses for epinio api server")
	}

	if len(ingresses.Items) < 1 {
		return "", "", errors.New("epinio api ingress not found")
	}

	if len(ingresses.Items) > 1 {
		return "", "", errors.New("more than one epinio api ingress found")
	}

	if len(ingresses.Items[0].Spec.Rules) < 1 {
		return "", "", errors.New("epinio api ingress has no rules")
	}

	if len(ingresses.Items[0].Spec.Rules) > 1 {
		return "", "", errors.New("epinio api ingress has more than on rule")
	}

	host := ingresses.Items[0].Spec.Rules[0].Host

	return fmt.Sprintf("%s://%s", epinioAPIProtocol, host), fmt.Sprintf("%s://%s", epinioWSProtocol, host), nil
}
