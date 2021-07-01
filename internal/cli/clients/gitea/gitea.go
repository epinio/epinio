// Package gitea deals with using gitea as a store for pushed applications and their deployment info
package gitea

import (
	"context"

	giteaSDK "code.gitea.io/sdk/gitea"
	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/auth"
	"github.com/pkg/errors"
)

// Client provides functionality for talking to a
// Gitea installation on Kubernetes
type Client struct {
	Client *giteaSDK.Client
	Auth   auth.PasswordAuth
}

const (
	GiteaCredentialsSecret = "gitea-creds"
)

var clientMemo *Client

// New loads the config and returns a new gitea client
func New(ctx context.Context) (*Client, error) {
	if clientMemo != nil {
		return clientMemo, nil
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	// See also deployments/gitea.go (service, func `apply`).
	// See also deployments/tekton.go, func `createGiteaCredsSecret`
	auth, err := getGiteaCredentials(ctx, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve gitea credentials")
	}

	client, err := giteaSDK.NewClient(deployments.GiteaURL)
	if err != nil {
		return nil, errors.Wrap(err, "gitea client creation failed")
	}

	client.SetBasicAuth(auth.Username, auth.Password)

	c := &Client{
		Client: client,
		Auth:   *auth,
	}

	clientMemo = c

	return c, nil
}

// getGiteaCredentials resolves Gitea's credentials
func getGiteaCredentials(ctx context.Context, cluster *kubernetes.Cluster) (*auth.PasswordAuth, error) {
	// See deployments/tekton.go, func `createGiteaCredsSecret`
	// for where `install` configures tekton for the credentials
	// retrieved here.
	//
	// See deployments/gitea.go func `apply` where `install`
	// configures gitea for the same credentials.
	s, err := cluster.GetSecret(ctx, deployments.TektonStagingNamespace, GiteaCredentialsSecret)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read gitea credentials")
	}

	username, ok := s.Data["username"]
	if !ok {
		return nil, errors.Wrap(err, "username key not found in gitea credentials secret")
	}

	password, ok := s.Data["password"]
	if !ok {
		return nil, errors.Wrap(err, "password key not found in gitea credentials secret")
	}

	return &auth.PasswordAuth{
		Username: string(username),
		Password: string(password),
	}, nil
}

func (c *Client) DeleteRepo(org, repo string) (int, error) {
	r, err := c.Client.DeleteRepo(org, repo)

	return r.StatusCode, err
}

func (c *Client) CreateOrg(org string) error {
	_, _, err := c.Client.CreateOrg(giteaSDK.CreateOrgOption{
		Name: org,
	})

	return err
}

func (c *Client) DeleteOrg(org string) error {
	_, err := c.Client.DeleteOrg(org)

	return err
}
