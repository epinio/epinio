package web

import (
	"net/http"

	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/version"
)

type InfoController struct {
}

func (hc InfoController) Index(w http.ResponseWriter, r *http.Request) {
	client, err := clients.NewEpinioClient(nil)
	if handleError(w, err, 500) {
		return
	}

	platform := client.KubeClient.GetPlatform()
	kubeVersion, err := client.KubeClient.GetVersion()
	if handleError(w, err, 500) {
		return
	}
	giteaVersion := "unavailable"
	giteaFetchedVersion, resp, err := client.GiteaClient.Client.ServerVersion()
	if err == nil && resp != nil && resp.StatusCode == 200 {
		giteaVersion = giteaFetchedVersion
	}

	data := map[string]interface{}{
		"version":      version.Version,
		"platform":     platform.String(),
		"kubeVersion":  kubeVersion,
		"giteaVersion": giteaVersion,
	}

	Render([]string{"main_layout", "icons", "info"}, w, r, data)
}
