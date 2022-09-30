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
