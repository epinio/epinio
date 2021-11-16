package web

import (
	"errors"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/gin-gonic/gin"
)

// NamespacesController represents all functionality of the dashboard related to namespaces
type NamespacesController struct {
}

// Target handles the dashboard's /namespaces/target endpoint. It verifies
// that the specified namespace exists, and then delivers a cookie
// persisting the targeted namespace in the browser state
func (hc NamespacesController) Target(c *gin.Context) {
	namespace := c.Param("namespace")
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if handleError(c, err) {
		return
	}

	exists, err := namespaces.Exists(ctx, cluster, namespace)
	if handleError(c, err) {
		return
	}

	if !exists {
		// When attempting to set an error into the response
		// caused an error, give up. The recovery middleware
		// will catch our panic and return that error.
		err := c.AbortWithError(
			http.StatusNotFound,
			errors.New("Namespace not found"),
		)
		if err != nil {
			panic(err.Error())
		}
		return
	}

	setCurrentNamespaceInCookie(namespace, "currentNamespace", c)

	c.Redirect(http.StatusFound, c.GetHeader("Referer"))
}
