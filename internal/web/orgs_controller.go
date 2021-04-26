package web

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/julienschmidt/httprouter"
)

type OrgsController struct {
}

func (hc OrgsController) Target(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := organizations.Exists(cluster, org)
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
