// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"context"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/duration"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubectl/pkg/util/podutils"
)

// Running handles the API endpoint GET /namespaces/:namespace/applications/:app/running
//
// It waits for the specified application to be running (i.e. its deployment to be
// complete), before it returns. An exception is if the application does not become ready
// within `duration.ToAppBuilt()` (default: 3 minutes). In that case it returns with an
// error after that time.
//
// __API PORTABILITY NOTE__
//
// With the switch to deployment of apps via helm, and waiting for the helm deployment to
// be ready before returning this endpoint is technically superfluous, and it should never
// fail. The command line has been modified to not invoke it anymore.
//
// It is kept for older clients still calling on it. Because of that it is also kept
// functional. Instead of checking for an app `Deployment` and its status it now checks
// the app `Pod` statuses.
func Running(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	if app.Workload == nil {
		// While the app exists it has no workload, and therefore no status
		return apierror.NewBadRequestError("no status available for application without workload")
	}

	// Check app readiness based on app pods. Wait only if we have non-ready pods.

	if app.Workload.DesiredReplicas != app.Workload.ReadyReplicas {
		err := wait.PollUntilContextTimeout(ctx, time.Second, duration.ToAppBuilt(), true, func(ctx context.Context) (bool, error) {
			podList, err := application.NewWorkload(cluster, app.Meta, app.Workload.DesiredReplicas).Pods(ctx)
			if err != nil {
				return false, err
			}
			var ready int32
			for _, pod := range podList {
				tmp := pod
				if podutils.IsPodReady(&tmp) {
					ready = ready + 1
				}
			}
			return ready == app.Workload.DesiredReplicas, nil
		})
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	response.OK(c)
	return nil
}
