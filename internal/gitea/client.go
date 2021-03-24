package gitea

import (
	"code.gitea.io/sdk/gitea"
	"github.com/pkg/errors"
)

// NewGiteaClient creates a new gitea client
// and performs authentication
func NewGiteaClient(resolver *Resolver) (*gitea.Client, error) {
	url, err := resolver.GetGiteaURL()
	if err != nil {
		return nil, errors.Wrap(err, "can't get gitea url")
	}

	client, err := gitea.NewClient(url)
	if err != nil {
		return nil, errors.Wrap(err, "gitea client failed")
	}

	username, password, err := resolver.GetGiteaCredentials()
	if err != nil {
		return nil, errors.Wrap(err, "can't determine gitea credentials")
	}

	client.SetBasicAuth(username, password)

	return client, nil
}
