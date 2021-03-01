package application

import (
	"errors"

	"code.gitea.io/sdk/gitea"
	"github.com/suse/carrier/kubernetes"
)

// Application manages applications.
// Implements the Application interface.
type Application struct {
	Name         string
	Organization string
	giteaClient  *gitea.Client
	kubeClient   *kubernetes.Cluster
}

type ApplicationList []Application

func (a *Application) Delete() error {
	// TODO: delete application
	// NOTE: has to do the things client does, without UI messages!
	// Hide the things (repo, hooks, whatnot, ...) from the user.
	return nil
}

func (a *Application) Bind(service string) error {
	// TODO PRIORITY. patch application deployment to use the service secret (derive from the org/service tuple).
	return nil
}

// Lookup locates an Application by org and name
func Lookup(
	kubeClient *kubernetes.Cluster,
	giteaClient *gitea.Client,
	org, app string) (Application, error) {

	apps, _, err := giteaClient.ListOrgRepos(org, gitea.ListOrgReposOptions{})
	if err != nil {
		return Application{}, err
	}

	for _, anApp := range apps {
		if anApp.Name == app {
			return Application{
				Organization: org,
				Name:         app,
				kubeClient:   kubeClient,
				giteaClient:  giteaClient,
			}, nil
		}
	}

	return Application{}, errors.New("Application not found")
}

// List returns an ApplicationList of all available applications (in the org)
func List(
	kubeClient *kubernetes.Cluster,
	giteaClient *gitea.Client,
	org string) (ApplicationList, error) {

	apps, _, err := giteaClient.ListOrgRepos(org, gitea.ListOrgReposOptions{})
	if err != nil {
		return nil, err
	}

	result := ApplicationList{}

	for _, app := range apps {
		result = append(result, Application{
			Organization: org,
			Name:         app.Name,
			kubeClient:   kubeClient,
			giteaClient:  giteaClient,
		})
	}

	return result, nil
}
