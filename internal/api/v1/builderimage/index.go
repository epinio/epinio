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

package builderimage

import (
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/builderimage"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Index handles GET /builderimages — lists all known builderimages.
func Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	client, clientError := cluster.ClientBuilderImage()
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	builderList, listError := builderimage.List(ctx, client)
	if listError != nil {
		return apierror.InternalError(listError)
	}

	appClient, appClientError := cluster.ClientApp()
	if appClientError != nil {
		return apierror.InternalError(appClientError)
	}

	inUse, inUseError := application.BuilderImagesInUse(ctx, appClient)
	if inUseError != nil {
		return apierror.InternalError(inUseError)
	}
	for i := range builderList {
		builderList[i].BoundApps = inUse[builderList[i].Image]
	}

	// Optional name filtering, applied before pagination so the page counts
	// describe the filtered set.
	search := response.GetSearchParam(c)
	if search != "" {
		lower := strings.ToLower(search)
		filtered := models.BuilderImageList{}

		for _, image := range builderList {
			name := strings.ToLower(image.Meta.Name)

			if strings.Contains(name, lower) {
				filtered = append(filtered, image)
			}
		}

		builderList = filtered
	}

	sortParams := response.GetSortParams(c)
	switch sortParams.Field {
	case "description":
		response.SortByStringKey([]models.BuilderImage(builderList), sortParams.Descending, func(image models.BuilderImage) string {
			return strings.ToLower(image.Description)
		})
	case "image":
		response.SortByStringKey([]models.BuilderImage(builderList), sortParams.Descending, func(image models.BuilderImage) string {
			return strings.ToLower(image.Image)
		})
	default:
		response.ApplyMetaSort([]models.BuilderImage(builderList), sortParams, response.MetaKeys[models.BuilderImage]{
			Name:      func(image models.BuilderImage) string { return image.Meta.Name },
			CreatedAt: func(image models.BuilderImage) time.Time { return image.Meta.CreatedAt.Time },
		})
	}

	page, pageSize, ok := response.GetPaginationParams(c, 1, 25)
	if ok {
		paged := response.PaginateSlice(builderList, page, pageSize)

		response.OKReturn(c, paged)
		return nil
	}

	response.OKReturn(c, builderList)
	return nil
}
