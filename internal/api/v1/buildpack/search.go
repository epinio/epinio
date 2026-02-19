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

package buildpack

import (
	"github.com/gin-gonic/gin"

	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Search handles GET /api/v1/buildpacks/search?q=<term>
// It searches the CNB registry index and returns matching buildpack entries.
func Search(c *gin.Context) apierror.APIErrors {
	q := c.Query("q")
	result, err := SearchCNBRegistry(c.Request.Context(), q)
	if err != nil {
		// Return empty result on error so UI doesn't break
		response.OKReturn(c, models.BuildpackSearchResponse{Buildpacks: []models.BuildpackEntry{}})
		return nil
	}
	response.OKReturn(c, *result)
	return nil
}
