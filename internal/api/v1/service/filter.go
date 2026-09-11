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

package service

import (
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// getCatalogServiceParam returns the optional `catalog_service` query
// parameter, narrowing a service list to the instances of a single catalog
// service.
func getCatalogServiceParam(c *gin.Context) string {
	return c.Query("catalog_service")
}

// filterServices narrows a service list by the optional `search` and
// `catalog_service` filters. A service has to pass both to survive; empty
// filters match everything.
//
// The search term is a case-insensitive substring of the instance name. The
// catalog service is matched exactly, because it names a resource -- a
// substring match would silently make the filter fuzzier than asked for.
func filterServices(
	list models.ServiceList,
	search, catalogService string,
) models.ServiceList {
	lowerSearch := strings.ToLower(search)

	// Never nil: a zero-match filter has to marshal to [] and not null.
	filtered := models.ServiceList{}

	for _, service := range list {
		name := strings.ToLower(service.Meta.Name)

		if lowerSearch != "" && !strings.Contains(name, lowerSearch) {
			continue
		}

		if catalogService != "" && service.CatalogService != catalogService {
			continue
		}

		filtered = append(filtered, service)
	}

	return filtered
}
