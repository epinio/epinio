package v1

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/version"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Info handles the API endpoint /info.  It returns version
// information for various epinio components.
func Info(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	kubeVersion, err := cluster.GetVersion()
	if err != nil {
		return InternalError(err)
	}

	platform := cluster.GetPlatform()

	info := models.InfoResponse{
		Version:     version.Version,
		Platform:    platform.String(),
		KubeVersion: kubeVersion,
	}

	err = response.JSON(w, info)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
