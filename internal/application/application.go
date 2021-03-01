package application

import (
	"errors"

	"code.gitea.io/sdk/gitea"
	"github.com/suse/carrier/internal/interfaces"
	"github.com/suse/carrier/kubernetes"
)

// CarrierApplication manages applications.
// Implements the Application interface.
type CarrierApplication struct {
	Application  string
	Organization string
	giteaClient  *gitea.Client
	kubeClient   *kubernetes.Cluster
}

func (a *CarrierApplication) Name() string {
	return a.Application
}

func (a *CarrierApplication) Org() string {
	return a.Organization
}

func (a *CarrierApplication) Delete() error {
	// TODO: delete application
	// NOTE: has to do the things client does, without UI messages!
	// Hide the things (repo, hooks, whatnot, ...) from the user.
	return nil
}

func (a *CarrierApplication) Bind(org, service string) error {
	// TODO PRIORITY. patch application deployment to use the service secret (derive from the org/service tuple).
	return nil
}

// Lookup locates an Application by org and name
func Lookup(
	kubeClient *kubernetes.Cluster,
	giteaClient *gitea.Client,
	org, app string) (interfaces.Application, error) {

	apps, _, err := giteaClient.ListOrgRepos(org, gitea.ListOrgReposOptions{})
	if err != nil {
		return nil, err
	}

	for _, anApp := range apps {
		if anApp.Name == app {
			return &CarrierApplication{
				Organization: org,
				Application:  app,
				kubeClient:   kubeClient,
				giteaClient:  giteaClient,
			}, nil
		}
	}

	return nil, errors.New("Application not found")
}

// List returns an ApplicationList of all available applications (in the org)
func List(
	kubeClient *kubernetes.Cluster,
	giteaClient *gitea.Client,
	org string) (interfaces.ApplicationList, error) {

	apps, _, err := giteaClient.ListOrgRepos(org, gitea.ListOrgReposOptions{})
	if err != nil {
		return nil, err
	}

	result := interfaces.ApplicationList{}

	for _, app := range apps {
		result = append(result, &CarrierApplication{
			Organization: org,
			Application:  app.Name,
			kubeClient:   kubeClient,
			giteaClient:  giteaClient,
		})
	}

	return result, nil
}
