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
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// FullIndex handles the API endpoint GET /applications
// It lists applications that have a deployed instance across namespaces.
// Rebuilds after deploy can also appear on UndeployedFullIndex.
func FullIndex(c *gin.Context) apierror.APIErrors {
	return fullIndexWithFilter(c, application.ListFilterDeployed)
}

// UndeployedFullIndex handles GET /applications/undeployed
// It lists apps with buildstatus create|build across namespaces the user can
// see, with optional pagination and search.
func UndeployedFullIndex(c *gin.Context) apierror.APIErrors {
	return fullIndexWithFilter(c, application.ListFilterUndeployed)
}

func fullIndexWithFilter(c *gin.Context, filter application.ListFilter) apierror.APIErrors {
	ctx := c.Request.Context()
	user := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	allCRs, err := application.ListAppCRs(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}

	search := response.GetSearchParam(c)
	matched := application.FilterAppCRs(allCRs, search, filter)
	matched = filterAppCRsByAccess(user, matched, response.GetNamespacesParam(c))

	page, pageSize, paginated := response.GetPaginationParams(c, 1, 25)
	if paginated {
		pageCRs, total := application.PageAppCRs(matched, page, pageSize)
		apps, err := application.AppsFromCRs(ctx, cluster, pageCRs)
		if err != nil {
			return apierror.InternalError(err)
		}
		response.OKReturn(c, response.BuildPaginatedResponse(apps, page, pageSize, total))
		return nil
	}

	apps, err := application.AppsFromCRs(ctx, cluster, matched)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, apps)
	return nil
}

// appCRNamespace adapts an App CR to auth.NamespacedResource so the shared auth
// helpers decide visibility, instead of a copy of their rules.
type appCRNamespace struct {
	unstructured.Unstructured
}

// Namespace returns the namespace of the wrapped App CR.
func (a appCRNamespace) Namespace() string {
	return a.GetNamespace()
}

// filterAppCRsByAccess keeps CRs the user may see, then optionally narrows to
// the namespaces query parameter (same rules as FilterResources + FilterByNamespaces).
func filterAppCRsByAccess(
	user auth.User,
	crs []unstructured.Unstructured,
	namespaces []string,
) []unstructured.Unstructured {
	wrapped := make([]appCRNamespace, 0, len(crs))
	for _, cr := range crs {
		wrapped = append(wrapped, appCRNamespace{cr})
	}

	wrapped = auth.FilterResources(user, wrapped)
	wrapped = auth.FilterByNamespaces(wrapped, namespaces)

	out := make([]unstructured.Unstructured, 0, len(wrapped))
	for _, w := range wrapped {
		out = append(out, w.Unstructured)
	}
	return out
}
