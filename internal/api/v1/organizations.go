package v1

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// OrganizationsController represents all functionality of the API related to namespaces
type OrganizationsController struct {
}

// Index handles the API endpoint /orgs
// It returns a list of all Epinio orgs.
// An Epinio org is nothing but a kubernetes namespace which has a
// special Label (Look at the code to see which).
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

// Create handles the API endpoint /orgs (POST).
// It creates the named organization.
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
		err := errors.New("name of namespace to create not found")
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
	_, err = w.Write([]byte{})
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Delete handles the API endpoint /orgs/:org.
// It destroys the named organization and all the applications and services in it.
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
		if err != nil && !apierrors.IsNotFound(err) {
			return InternalError(err)
		}
	}

	// Deleting the namespace here. That will automatically delete the application resources.
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

// deleteApps removes the application and its resources
func deleteApps(ctx context.Context, cluster *kubernetes.Cluster, gitea *gitea.Client, org string) error {
	appRefs, err := application.ListAppRefs(ctx, cluster, org)
	if err != nil {
		return err
	}

	// Operation of the concurrent code below:
	//
	// 1. A wait group `wg` is used to ensure that the main thread
	//    of the function does not return until all deletions in
	//    flight have completed, one way or other (z). The
	//    dispatch loop expands the wait group (1a), each
	//    completed runner shrinks it, via defer (1b).
	//
	// 2. The `buffer` channel is used to control and limit the
	//    amount of concurrency. Each iteration of the dispatch
	//    loop enters a signal into the channel (2a), blocking
	//    when the capacity (= concurrency limit) is reached, or
	//    spawning a runner. Runners remove signals from the
	//    channel as they complete (2b), freeing up capacity and
	//    unblocking the dispatcher.
	//
	// 3. The error handling is a bit tricky, as it has to take
	//    two cases into account, about the timeline of events
	//    happening:
	//
	//    a. If even a single runner was fast enough to report an
	//       error (x) while the dispatch loop is still running,
	//       then that error is captured by the loop itself, at
	//       (3a1) and then reported at (3a2), after the other
	//       runners in flight have completed also. The loop also
	//       stops dispatching more runners.
	//
	//     b. If on the other hand the dispatch loop completed
	//        before any runner reported an error, then that error
	//        is captured and reported at (3b1).
	//
	//        This part works because
	//
	//        i. The command waiting for all runners to complete
	//           (z) ensures that an empty channel means that no
	//           errors occurred, there can be no stragglers to
	//           wait for at (3b1).
	//
	//        ii. The error channel has capacity according to the
	//            concurrency limit, i.e. enough space to capture
	//            the errors from all possible runners, without
	//            blocking any of them from completion, and thus
	//            not block the wait group either at (z).

	const maxConcurrent = 100
	buffer := make(chan struct{}, maxConcurrent)
	errChan := make(chan error, maxConcurrent)
	var wg sync.WaitGroup
	var forLoopErr error

loop:
	for _, appRef := range appRefs {
		buffer <- struct{}{} // 2a
		wg.Add(1)            // 1a

		go func(appRef models.AppRef) {
			defer wg.Done() // 1b
			defer func() {
				<-buffer // 2b
			}()
			err := application.Delete(ctx, cluster, gitea, appRef)
			if err != nil {
				errChan <- err // x
			}
		}(appRef)

		// 3a1
		select {
		case forLoopErr = <-errChan:
			break loop
		default:
		}
	}
	wg.Wait() // z

	// 3a2
	if forLoopErr != nil {
		return forLoopErr
	}

	// 3b1
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}

}
