package v1

import (
	"encoding/json"
	"net/http"

	giteaSDK "code.gitea.io/sdk/gitea"
	"github.com/suse/carrier/internal/cli/clients"
)

type OrganisationsController struct {
}

func (oc OrganisationsController) Index(w http.ResponseWriter, r *http.Request) {
	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, 500) {
		return
	}

	var orgList []string
	orgs, _, err := gitea.Client.AdminListOrgs(giteaSDK.AdminListOrgsOptions{})
	if handleError(w, err, 500) {
		return
	}

	for _, org := range orgs {
		orgList = append(orgList, org.UserName)
	}

	js, err := json.Marshal(orgList)
	if handleError(w, err, 500) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
