package web

import (
	"net/http"

	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/julienschmidt/httprouter"
)

type OrgsController struct {
}

func (hc OrgsController) Target(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	availableOrgs, err := gitea.OrgNames()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	orgExists := func(lookupOrg string, orgs []string) bool {
		for _, org := range orgs {
			if org == lookupOrg {
				return true
			}
		}
		return false
	}(org, availableOrgs)

	if !orgExists {
		http.Error(w, "Organization not found", 404)
		return
	}

	setCurrentOrgInCookie(org, "currentOrg", w)

	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
}
