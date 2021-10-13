package web

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/version"
	"github.com/gin-gonic/gin"
)

// InfoController represents all functionality of the dashboard related to epinio inspection
type InfoController struct {
}

// Index handles the dashboard's /info endpoint. It returns version information for various epinio components.
func (hc InfoController) Index(c *gin.Context) {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if handleError(c, err) {
		return
	}

	platform := cluster.GetPlatform()
	kubeVersion, err := cluster.GetVersion()
	if handleError(c, err) {
		return
	}

	data := map[string]interface{}{
		"version":     version.Version,
		"platform":    platform.String(),
		"kubeVersion": kubeVersion,
	}

	Render([]string{
		"main_layout",
		"icons",
		"info",
		"modals",
	}, c, data)
}
