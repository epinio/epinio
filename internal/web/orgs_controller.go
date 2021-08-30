package web

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/julienschmidt/httprouter"
)

// OrgsController represents all functionality of the dashboard related to organizations
type OrgsController struct {
}

// Target handles the dashboard's /orgs/target endpoint. It verifies
// that the specified org exists, and then delivers a cookie
// persisting the targeted org in the browser state
func (hc OrgsController) Target(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster(ctx)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	if !exists {
		http.Error(w, "Organization not found", 404)
		return
	}

	setCurrentOrgInCookie(org, "currentOrg", w)

	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
}
