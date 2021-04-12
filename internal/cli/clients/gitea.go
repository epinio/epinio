package clients

import (
	"fmt"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/internal/cli/config"
	"github.com/epinio/epinio/kubernetes"
	"github.com/pkg/errors"
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

// Resolver figures out where Gitea lives and how to login to it
type Resolver struct {
	cluster *kubernetes.Cluster
	config  *config.Config
}

const (
	GiteaCredentialsSecret = "gitea-creds"
)

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

	domain, err := getMainDomain(cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine epinio domain")
	}

	giteaURL, err := getGiteaURL(configConfig, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve gitea host")
	}

	username, password, err := getGiteaCredentials(cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve gitea credentials")
	}

	client, err := gitea.NewClient(giteaURL)
	if err != nil {
		return nil, errors.Wrap(err, "gitea client failed")
	}
	client.SetBasicAuth(username, password)

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

// getMainDomain finds the main domain for Epinio
func getMainDomain(cluster *kubernetes.Cluster) (string, error) {
	// Get the ingress
	ingresses, err := cluster.ListIngress(deployments.GiteaDeploymentID, "app.kubernetes.io/name=gitea")
	if err != nil {
		return "", errors.Wrap(err, "failed to list ingresses for gitea")
	}

	if len(ingresses.Items) < 1 {
		return "", errors.New("gitea ingress not found")
	}

	if len(ingresses.Items) > 1 {
		return "", errors.New("more than one gitea ingress found")
	}

	if len(ingresses.Items[0].Spec.Rules) < 1 {
		return "", errors.New("gitea ingress has no rules")
	}

	if len(ingresses.Items[0].Spec.Rules) > 1 {
		return "", errors.New("gitea ingress has more than on rule")
	}

	host := ingresses.Items[0].Spec.Rules[0].Host

	return strings.TrimPrefix(host, "gitea."), nil
}

// getGiteaURL finds the URL for gitea
func getGiteaURL(config *config.Config, cluster *kubernetes.Cluster) (string, error) {
	// Get the ingress
	ingresses, err := cluster.ListIngress(deployments.GiteaDeploymentID, "app.kubernetes.io/name=gitea")
	if err != nil {
		return "", errors.Wrap(err, "failed to list ingresses for gitea")
	}

	if len(ingresses.Items) < 1 {
		return "", errors.New("gitea ingress not found")
	}

	if len(ingresses.Items) > 1 {
		return "", errors.New("more than one gitea ingress found")
	}

	if len(ingresses.Items[0].Spec.Rules) < 1 {
		return "", errors.New("gitea ingress has no rules")
	}

	if len(ingresses.Items[0].Spec.Rules) > 1 {
		return "", errors.New("gitea ingress has more than on rule")
	}

	host := ingresses.Items[0].Spec.Rules[0].Host

	return fmt.Sprintf("%s://%s", config.GiteaProtocol, host), nil
}

// getGiteaCredentials resolves Gitea's credentials
func getGiteaCredentials(cluster *kubernetes.Cluster) (string, string, error) {
	s, err := cluster.GetSecret(deployments.WorkloadsDeploymentID, GiteaCredentialsSecret)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to read gitea credentials")
	}

	username, ok := s.Data["username"]
	if !ok {
		return "", "", errors.Wrap(err, "username key not found in gitea credentials secret")
	}

	password, ok := s.Data["password"]
	if !ok {
		return "", "", errors.Wrap(err, "password key not found in gitea credentials secret")
	}

	return string(username), string(password), nil
}

func (c *GiteaClient) OrgExists(org string) (bool, error) {
	_, resp, err := c.Client.GetOrg(org)
	// if gitea sends us a 404 it's both an error and a response with 404.
	// We handle that below.
	if resp == nil && err != nil {
		return false, errors.Wrap(err, "failed to make get org request")
	}

	if resp.StatusCode == 404 {
		return false, nil
	}

	if resp.StatusCode != 200 {
		return false, errors.Errorf("Unexpected response from Gitea: %d", resp.StatusCode)
	}

	return true, nil
}
