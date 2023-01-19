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
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/version"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

const VersionHeader = "epinio-version"

// Info handles the API endpoint /info.  It returns version
// information for various epinio components.
func Info(c *gin.Context) APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	kubeVersion, err := cluster.GetVersion()
	if err != nil {
		return InternalError(err)
	}

	platform := cluster.GetPlatform()

	stageConfig, err := cluster.GetConfigMap(ctx, helmchart.Namespace(), helmchart.EpinioStageScriptsName)
	if err != nil {
		return InternalError(err, "failed to retrieve staging image refs")
	}

	defaultBuilderImage := stageConfig.Data["builderImage"]

	response.OKReturn(c, models.InfoResponse{
		Version:             version.Version,
		Platform:            platform.String(),
		KubeVersion:         kubeVersion,
		DefaultBuilderImage: defaultBuilderImage,
	})
	return nil
}
