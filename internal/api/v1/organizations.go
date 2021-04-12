package v1

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	giteaSDK "code.gitea.io/sdk/gitea"
	"github.com/epinio/epinio/internal/cli/clients"
)

type OrganizationsController struct {
}

func (oc OrganizationsController) Index(w http.ResponseWriter, r *http.Request) {
	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	// TODO: Wrap AdminListOrgs into a local gitea client method (See OrgExists)
	orgs, _, err := gitea.Client.AdminListOrgs(giteaSDK.AdminListOrgsOptions{})
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	var orgList []string
	for _, org := range orgs {
		orgList = append(orgList, org.UserName)
	}

	js, err := json.Marshal(orgList)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func (oc OrganizationsController) Create(w http.ResponseWriter, r *http.Request) {
	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	// map ~ json oject / Required key: name
	var parts map[string]string
	err = json.Unmarshal(bodyBytes, &parts)
	if handleError(w, err, http.StatusBadRequest) {
		return
	}

	org, ok := parts["name"]
	if !ok {
		http.Error(w, fmt.Sprintf("Name of organization to create not found"),
			http.StatusBadRequest)
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if exists {
		http.Error(w, fmt.Sprintf("Organization '%s' already exists", org),
			http.StatusConflict)
		return
	}

	// TODO: Wrap CreateOrg into a local gitea client method (See OrgExists)
	_, _, err = gitea.Client.CreateOrg(giteaSDK.CreateOrgOption{
		Name: org,
	})
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte{})
}
