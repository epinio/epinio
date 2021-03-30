package clients

import (
	"code.gitea.io/sdk/gitea"
	"github.com/pkg/errors"
	"github.com/suse/carrier/internal/cli/config"
	carriergitea "github.com/suse/carrier/internal/gitea"
	"github.com/suse/carrier/kubernetes"
)

// GiteaClient provides functionality for talking to a
// Gitea installation on Kubernetes
type GiteaClient struct {
	Client   *gitea.Client
	Domain   string
	URL      string
	Username string
	Password string
}

var giteaClientMemo *GiteaClient

func GetGiteaClient() (*GiteaClient, error) {
	if giteaClientMemo != nil {
		return giteaClientMemo, nil
	}

	configConfig, err := config.Load()
	if err != nil {
		return nil, err
	}

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return nil, err
	}

	resolver := carriergitea.NewResolver(configConfig, cluster)

	domain, err := resolver.GetMainDomain()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine carrier domain")
	}

	giteaURL, err := resolver.GetGiteaURL()
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve gitea host")
	}

	username, password, err := resolver.GetGiteaCredentials()
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve gitea credentials")
	}

	client, err := carriergitea.NewGiteaClient(resolver)
	if err != nil {
		return nil, err
	}

	giteaClient := &GiteaClient{
		Client:   client,
		Domain:   domain,
		URL:      giteaURL,
		Username: username,
		Password: password,
	}

	giteaClientMemo = giteaClient

	return giteaClient, nil
}
