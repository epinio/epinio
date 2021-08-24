package web

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/version"
)

// InfoController represents all functionality of the dashboard related to epinio inspection
type InfoController struct {
}

// Index handles the dashboard's /info endpoint. It returns version information for various epinio components.
func (hc InfoController) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if handleError(w, err, 500) {
		return
	}

	platform := cluster.GetPlatform()
	kubeVersion, err := cluster.GetVersion()
	if handleError(w, err, 500) {
		return
	}

	data := map[string]interface{}{
		"version":     version.Version,
		"platform":    platform.String(),
		"kubeVersion": kubeVersion,
	}

	Render([]string{"main_layout", "icons", "info", "modals"}, w, r, data)
}
