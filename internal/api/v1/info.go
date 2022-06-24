package v1

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/version"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Info handles the API endpoint /info.  It returns version
// information for various epinio components.
func Info(c *gin.Context) APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return NewInternalError(err)
	}

	kubeVersion, err := cluster.GetVersion()
	if err != nil {
		return NewInternalError(err)
	}

	platform := cluster.GetPlatform()

	response.OKReturn(c, models.InfoResponse{
		Version:     version.Version,
		Platform:    platform.String(),
		KubeVersion: kubeVersion,
	})
	return nil
}
