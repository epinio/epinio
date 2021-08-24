package web

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/version"
)

type InfoController struct {
}

func (hc InfoController) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	giteaClient, err := gitea.New(ctx)
	if handleError(w, err, 500) {
		return
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if handleError(w, err, 500) {
		return
	}

	platform := cluster.GetPlatform()
	kubeVersion, err := cluster.GetVersion()
	if handleError(w, err, 500) {
		return
	}
	giteaVersion := "unavailable"
	giteaFetchedVersion, resp, err := giteaClient.Client.ServerVersion()
	if err == nil && resp != nil && resp.StatusCode == 200 {
		giteaVersion = giteaFetchedVersion
	}

	data := map[string]interface{}{
		"version":      version.Version,
		"platform":     platform.String(),
		"kubeVersion":  kubeVersion,
		"giteaVersion": giteaVersion,
	}

	Render([]string{"main_layout", "icons", "info", "modals"}, w, r, data)
}
