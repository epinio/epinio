package namespace

import (
	"context"
	"sync"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Delete handles the API endpoint /namespaces/:org (DELETE).
// It destroys the namespace specified by its name.
// This includes all the applications and services in it.
func (oc Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	org := c.Param("org")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}
	if !exists {
		return apierror.OrgIsNotKnown(org)
	}

	err = deleteApps(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	serviceList, err := services.List(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	for _, service := range serviceList {
		err = service.Delete(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return apierror.InternalError(err)
		}
	}

	// Deleting the namespace here. That will automatically delete the application resources.
	err = organizations.Delete(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
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
