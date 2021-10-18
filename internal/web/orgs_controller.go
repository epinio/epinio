package web

import (
	"errors"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/gin-gonic/gin"
)

// OrgsController represents all functionality of the dashboard related to organizations
type OrgsController struct {
}

// Target handles the dashboard's /orgs/target endpoint. It verifies
// that the specified org exists, and then delivers a cookie
// persisting the targeted org in the browser state
func (hc OrgsController) Target(c *gin.Context) {
	org := c.Param("org")
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if handleError(c, err) {
		return
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if handleError(c, err) {
		return
	}

	if !exists {
		// When attempting to set an error into the response
		// caused an error, give up. The recovery middleware
		// will catch our panic and return that error.
		err := c.AbortWithError(
			http.StatusNotFound,
			errors.New("Organization not found"),
		)
		if err != nil {
			panic(err.Error())
		}
		return
	}

	setCurrentOrgInCookie(org, "currentOrg", c)

	c.Redirect(http.StatusFound, c.GetHeader("Referer"))
}
