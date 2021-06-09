package v1

import (
	"context"
	"encoding/json"
	"errors"
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
	ctx := r.Context()
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	orgList, err := organizations.List(ctx, cluster)
	if err != nil {
		return InternalError(err)
	}

	orgNames := []string{}
	for _, org := range orgList {
		orgNames = append(orgNames, org.Name)
	}

	js, err := json.Marshal(orgNames)
	if err != nil {
		return InternalError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func (oc OrganizationsController) Create(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	gitea, err := gitea.New(ctx)
	if err != nil {
		return InternalError(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	// map ~ json oject / Required key: name
	var parts map[string]string
	err = json.Unmarshal(bodyBytes, &parts)
	if err != nil {
		return BadRequest(err)
	}

	org, ok := parts["name"]
	if !ok {
		err := errors.New("Name of organization to create not found")
		return BadRequest(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if exists {
		return OrgAlreadyKnown(org)
	}

	err = organizations.Create(r.Context(), cluster, gitea, org)
	if err != nil {
		return InternalError(err)
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte{})

	return nil
}

func (oc OrganizationsController) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	gitea, err := gitea.New(ctx)
	if err != nil {
		return InternalError(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	err = deleteApps(ctx, cluster, gitea, org)
	if err != nil {
		return InternalError(err)
	}

	serviceList, err := services.List(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	for _, service := range serviceList {
		err = service.Delete(ctx)
		if err != nil {
			return InternalError(err)
		}
	}

	err = organizations.Delete(ctx, cluster, gitea, org)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte{})
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func deleteApps(ctx context.Context, cluster *kubernetes.Cluster, gitea *gitea.Client, org string) error {
	apps, err := application.List(ctx, cluster, org)
	if err != nil {
		return err
	}

	// create a buffer, so there is no more than 'maxConcurrent' goroutines running at the same time
	const maxConcurrent = 100
	buffer := make(chan struct{}, maxConcurrent)
	errChan := make(chan error, maxConcurrent)
	var wg sync.WaitGroup
	var forLoopErr error

loop:
	for _, app := range apps {
		buffer <- struct{}{}
		wg.Add(1)

		go func(app application.Application) {
			defer wg.Done()
			defer func() {
				<-buffer
			}()
			err = application.Delete(ctx, cluster, gitea, org, app)
			if err != nil {
				errChan <- err
			}
		}(app)

		select {
		case forLoopErr = <-errChan:
			break loop
		default:
		}
	}
	wg.Wait()

	if forLoopErr != nil {
		return forLoopErr
	}

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}

}
