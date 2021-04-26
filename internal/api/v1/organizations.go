package v1

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/organizations"
)

type OrganizationsController struct {
}

// Index return a list of all Epinio orgs
// An Epinio org is nothing but a kubernetes namespace which has a special
// Label (Look at the code to see which).
func (oc OrganizationsController) Index(w http.ResponseWriter, r *http.Request) {
	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	orgList, err := organizations.List(cluster)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	orgNames := []string{}
	for _, org := range orgList {
		orgNames = append(orgNames, org.Name)
	}

	js, err := json.Marshal(orgNames)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (oc OrganizationsController) Create(w http.ResponseWriter, r *http.Request) {
	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	cluster, err := kubernetes.GetCluster()
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

	exists, err := organizations.Exists(cluster, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if exists {
		http.Error(w, fmt.Sprintf("Organization '%s' already exists", org),
			http.StatusConflict)
		return
	}

	err = organizations.Create(cluster, gitea, org)

	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte{})
}
