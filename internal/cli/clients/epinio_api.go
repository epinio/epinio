package clients

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/pkg/errors"
)

const (
	epinioAPIProtocol = "https"
	epinioWSProtocol  = "wss"
)

// EpinioAPIClient provides functionality for talking to an Epinio API
// server on Kubernetes
type EpinioAPIClient struct {
	URL   string
	WsURL string
}

var epinioClientMemo *EpinioAPIClient

func GetEpinioAPIClient(ctx context.Context) (*EpinioAPIClient, error) {
	if epinioClientMemo != nil {
		return epinioClientMemo, nil
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	epinioURL, epinioWsURL, err := getEpinioURL(ctx, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve epinio api host")
	}

	epinioClient := &EpinioAPIClient{
		URL:   epinioURL,
		WsURL: epinioWsURL,
	}

	epinioClientMemo = epinioClient

	return epinioClient, nil
}

// getEpinioURL finds the URL's for epinio
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
