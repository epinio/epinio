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

package v1

import (
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"

	"github.com/gin-gonic/gin"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Me handles the API endpoint /me.
// It returns some information about the current user
func Me(c *gin.Context) APIErrors {
	user := requestctx.User(c.Request.Context())

	roles := []models.Role{}
	for _, r := range user.Roles {
		roles = append(roles, models.Role{
			ID:        r.ID,
			Name:      r.Name,
			Namespace: r.Namespace,
			Default:   r.Default,
		})
	}

	response.OKReturn(c, models.MeResponse{
		User:       user.Username,
		Roles:      roles,
		Namespaces: user.Namespaces,
		Gitconfigs: user.Gitconfigs,
	})
	return nil
}
