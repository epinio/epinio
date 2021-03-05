package gitea

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/kubernetes"
	"github.com/suse/carrier/paas/config"
)

const (
	GiteaCredentialsSecret = "gitea-creds"
)

// Resolver figures out where Gitea lives and how to login to it
type Resolver struct {
	cluster *kubernetes.Cluster
	config  *config.Config
}

// NewResolver creates a new Resolver
func NewResolver(config *config.Config, cluster *kubernetes.Cluster) *Resolver {
	return &Resolver{
		cluster: cluster,
		config:  config,
	}
}

// GetMainDomain finds the main domain for Carrier
func (r *Resolver) GetMainDomain() (string, error) {
	// Get the ingress
	ingresses, err := r.cluster.ListIngress(deployments.GiteaDeploymentID, "app.kubernetes.io/name=gitea")
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

// GetGiteaURL finds the URL for gitea
func (r *Resolver) GetGiteaURL() (string, error) {
	// Get the ingress
	ingresses, err := r.cluster.ListIngress(deployments.GiteaDeploymentID, "app.kubernetes.io/name=gitea")
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

	return fmt.Sprintf("%s://%s", r.config.GiteaProtocol, host), nil
}

// GetGiteaCredentials resolves Gitea's credentials
func (r *Resolver) GetGiteaCredentials() (string, string, error) {
	s, err := r.cluster.GetSecret(r.config.CarrierWorkloadsNamespace, GiteaCredentialsSecret)
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
