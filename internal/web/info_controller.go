package web

import (
	"net/http"

	"github.com/suse/carrier/paas"
	"github.com/suse/carrier/version"
)

type InfoController struct {
}

func (hc InfoController) Index(w http.ResponseWriter, r *http.Request) {
	// TODO: Second return value is always nil (the cleanup function)?
	client, _, err := paas.NewCarrierClient(nil)
	if handleError(w, err, 500) {
		return
	}

	platform := client.KubeClient.GetPlatform()
	kubeVersion, err := client.KubeClient.GetVersion()
	if handleError(w, err, 500) {
		return
	}
	giteaVersion := "unavailable"
	giteaFetchedVersion, resp, err := client.GiteaClient.ServerVersion()
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
