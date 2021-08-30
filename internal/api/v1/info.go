package v1

import (
	"encoding/json"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
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

	info := struct {
		Version     string
		KubeVersion string
		Platform    string
	}{
		Version:     version.Version,
		Platform:    platform.String(),
		KubeVersion: kubeVersion,
	}

	js, err := json.Marshal(info)
	if err != nil {
		return InternalError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
