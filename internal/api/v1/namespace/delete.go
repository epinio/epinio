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

package namespace

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	ants "github.com/panjf2000/ants/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Delete handles the API endpoint /namespaces/:namespace (DELETE).
// It destroys the namespace specified by its name.
// This includes all the applications and configurations in it.
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx)
	namespaceName := c.Param("namespace")

	var namespaceNames []string
	namespaceNames, found := c.GetQueryArray("namespaces[]")
	if !found {
		namespaceNames = append(namespaceNames, namespaceName)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	authService := auth.NewAuthService(logger, cluster)

	for _, namespace := range namespaceNames {
		err = deleteApps(ctx, cluster, namespace)
		if err != nil {
			return apierror.InternalError(err)
		}

		err = deleteServices(ctx, cluster, namespace)
		if err != nil {
			return apierror.InternalError(err)
		}

		// delete the namespace from all the Users
		err = authService.RemoveNamespaceFromUsers(ctx, namespace)
		if err != nil {
			errDetail := fmt.Sprintf("error removing namespace [%s] from users", namespace)
			return apierror.InternalError(err, errDetail)
		}

		configurationList, err := configurations.List(ctx, cluster, namespace)
		if err != nil {
			return apierror.InternalError(err)
		}

		for _, configuration := range configurationList {
			err = configuration.Delete(ctx)
			if err != nil && !apierrors.IsNotFound(err) {
				return apierror.InternalError(err)
			}
		}

		// Deleting the namespace here. That will automatically delete the application resources.
		err = namespaces.Delete(ctx, cluster, namespace)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	response.OK(c)
	return nil
}

// deleteApps removes the application and its resources
func deleteApps(ctx context.Context, cluster *kubernetes.Cluster, namespace string) error {
	appRefs, err := application.ListAppRefs(ctx, cluster, namespace)
	if err != nil {
		return err
	}

	const maxConcurrent = 100
	errChan := make(chan error)

	var wg, errWg sync.WaitGroup
	var loopErrs []error

	errWg.Add(1)
	go func() {
		for err := range errChan {
			loopErrs = append(loopErrs, err)
		}
		errWg.Done()
	}()

	p, err := ants.NewPoolWithFunc(maxConcurrent, func(i interface{}) {
		err := application.Delete(ctx, cluster, i.(models.AppRef))
		if err != nil {
			errChan <- err
		}
		wg.Done()
	}, ants.WithExpiryDuration(10*time.Second))
	if err != nil {
		return err
	}

	for _, appRef := range appRefs {
		wg.Add(1)
		err = p.Invoke(appRef)
		if err != nil {
			errChan <- err
		}
	}
	defer p.Release()

	wg.Wait()
	close(errChan)
	errWg.Wait()

	totalErrs := len(loopErrs)
	if totalErrs > 0 {
		return errors.Wrapf(loopErrs[1], "%d errors occurred. This is the first one", totalErrs)
	}

	return nil
}

// deleteServices removes all provisioned services when a Namespace is deleted
func deleteServices(ctx context.Context, cluster *kubernetes.Cluster, namespace string) error {
	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	return kubeServiceClient.DeleteAll(ctx, namespace)
}
