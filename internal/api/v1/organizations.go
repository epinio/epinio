package v1

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/organizations"
)

type OrganizationsController struct {
}

// Index return a list of all Epinio orgs
// An Epinio org is nothing but a kubernetes namespace which has a special
// Label (Look at the code to see which).
func (oc OrganizationsController) Index(w http.ResponseWriter, r *http.Request) []APIError {
	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	orgList, err := organizations.List(cluster)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	orgNames := []string{}
	for _, org := range orgList {
		orgNames = append(orgNames, org.Name)
	}

	js, err := json.Marshal(orgNames)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return []APIError{}
}

func (oc OrganizationsController) Create(w http.ResponseWriter, r *http.Request) []APIError {
	gitea, err := gitea.New()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	// map ~ json oject / Required key: name
	var parts map[string]string
	err = json.Unmarshal(bodyBytes, &parts)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusBadRequest)}
	}

	org, ok := parts["name"]
	if !ok {
		return []APIError{NewAPIError("Name of organization to create not found", "", http.StatusBadRequest)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}
	if exists {
		return []APIError{
			NewAPIError(fmt.Sprintf("Organization '%s' already exists", org), "", http.StatusConflict),
		}
	}

	err = organizations.Create(r.Context(), cluster, gitea, org)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte{})

	return []APIError{}
}
