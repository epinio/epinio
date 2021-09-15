package v1

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/version"
)

// InfoController represents all functionality of the API related to epinio inspection
type InfoController struct {
}

// Info handles the API endpoint /info.  It returns version
// information for various epinio components.
func (hc InfoController) Info(w http.ResponseWriter, r *http.Request) APIErrors {
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

	err = jsonResponse(w, info)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
