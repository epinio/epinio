package v1

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// NamespacesController represents all functionality of the API related to namespaces
type NamespacesController struct {
}

// Match handles the API endpoint /namespaces/:pattern (GET)
// It returns a list of all Epinio-controlled namespaces matching the prefix pattern.
func (oc NamespacesController) Match(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	log.Info("match namespaces")
	defer log.Info("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	log.Info("list namespaces")
	namespaces, err := organizations.List(ctx, cluster)
	if err != nil {
		return InternalError(err)
	}

	log.Info("get namespace prefix")
	params := httprouter.ParamsFromContext(ctx)
	prefix := params.ByName("pattern")

	log.Info("match prefix", "pattern", prefix)
	matches := []string{}
	for _, namespace := range namespaces {
		if strings.HasPrefix(namespace.Name, prefix) {
			matches = append(matches, namespace.Name)
		}
	}

	log.Info("deliver matches", "found", matches)

	err = jsonResponse(w, models.NamespacesMatchResponse{Names: matches})
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Index handles the API endpoint /namespaces (GET)
// It returns a list of all Epinio-controlled namespaces
// An Epinio namespace is nothing but a kubernetes namespace which has a
// special Label (Look at the code to see which).
func (oc NamespacesController) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	orgList, err := organizations.List(ctx, cluster)
	if err != nil {
		return InternalError(err)
	}

	namespaces := make(models.NamespaceList, 0, len(orgList))
	for _, org := range orgList {
		// Retrieve app references for namespace, and reduce to their names.
		appRefs, err := application.ListAppRefs(ctx, cluster, org.Name)
		if err != nil {
			return InternalError(err)
		}
		appNames := make([]string, 0, len(appRefs))
		for _, app := range appRefs {
			appNames = append(appNames, app.Name)
		}

		// Retrieve services for namespace, and reduce to their names.
		services, err := services.List(ctx, cluster, org.Name)
		if err != nil {
			return InternalError(err)
		}
		serviceNames := make([]string, 0, len(services))
		for _, service := range services {
			serviceNames = append(serviceNames, service.Name())
		}

		namespaces = append(namespaces, models.Namespace{
			Name:     org.Name,
			Apps:     appNames,
			Services: serviceNames,
		})
	}

	err = jsonResponse(w, namespaces)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Create handles the API endpoint /namespaces (POST).
// It creates a namespace with the specified name.
func (oc NamespacesController) Create(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()

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

	err = organizations.Create(r.Context(), cluster, org)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = jsonResponse(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Delete handles the API endpoint /namespaces/:org (DELETE).
// It destroys the namespace specified by its name.
// This includes all the applications and services in it.
func (oc NamespacesController) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

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

	err = deleteApps(ctx, cluster, org)
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
	err = organizations.Delete(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = jsonResponse(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// deleteApps removes the application and its resources
func deleteApps(ctx context.Context, cluster *kubernetes.Cluster, org string) error {
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
			err := application.Delete(ctx, cluster, appRef)
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
