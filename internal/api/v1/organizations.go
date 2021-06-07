package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

type OrganizationsController struct {
}

// Index return a list of all Epinio orgs
// An Epinio org is nothing but a kubernetes namespace which has a special
// Label (Look at the code to see which).
func (oc OrganizationsController) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	orgList, err := organizations.List(cluster)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	orgNames := []string{}
	for _, org := range orgList {
		orgNames = append(orgNames, org.Name)
	}

	js, err := json.Marshal(orgNames)
	if err != nil {
		return APIErrors{InternalError(err)}
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	return nil
}

func (oc OrganizationsController) Create(w http.ResponseWriter, r *http.Request) APIErrors {
	gitea, err := gitea.New()
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	// map ~ json oject / Required key: name
	var parts map[string]string
	err = json.Unmarshal(bodyBytes, &parts)
	if err != nil {
		return APIErrors{BadRequest(err)}
	}

	org, ok := parts["name"]
	if !ok {
		err := errors.New("Name of organization to create not found")
		return APIErrors{BadRequest(err)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return APIErrors{InternalError(err)}
	}
	if exists {
		return APIErrors{OrgAlreadyKnown(org)}
	}

	err = organizations.Create(r.Context(), cluster, gitea, org)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte{})

	return nil
}

func (oc OrganizationsController) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	gitea, err := gitea.New()
	if err != nil {
		return APIErrors{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return APIErrors{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return APIErrors{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}
	if !exists {
		return APIErrors{
			NewAPIError(fmt.Sprintf("Organization '%s' does not exist", org), "", http.StatusNotFound),
		}
	}

	err = deleteApps(cluster, gitea, org)
	if err != nil {
		return APIErrors{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	serviceList, err := services.List(cluster, org)
	if err != nil {
		return APIErrors{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	for _, service := range serviceList {
		err = service.Delete()
		if err != nil {
			return APIErrors{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
		}
	}

	err = organizations.Delete(r.Context(), cluster, gitea, org)
	if err != nil {
		return APIErrors{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte{})
	if err != nil {
		return APIErrors{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return nil
}

func deleteApps(cluster *kubernetes.Cluster, gitea *gitea.Client, org string) error {
	apps, err := application.List(cluster, org)
	if err != nil {
		return err
	}

	const maxConcurrent = 100
	errChan := make(chan error, maxConcurrent)
	success := make(chan struct{})
	stop := make(chan struct{})

	go func() {
		var wg sync.WaitGroup
		// create a buffer, so there is no more than 'maxConcurrent' goroutines running at the same time
		buffer := make(chan struct{}, maxConcurrent)
	loop:
		for _, app := range apps {
			buffer <- struct{}{}

			go func(app application.Application) {
				defer wg.Done()
				wg.Add(1)
				err = application.Delete(cluster, gitea, org, app)
				if err != nil {
					errChan <- err
				}
				<-buffer
			}(app)

			select {
			case <-stop:
				break loop
			default:
			}
		}
		wg.Wait()
		success <- struct{}{}
	}()

	//wait until all apps are deleted or we get an error
	select {
	case err := <-errChan:
		stop <- struct{}{}
		return err
	case <-success:
		return nil
	}

}
