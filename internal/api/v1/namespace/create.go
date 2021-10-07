package namespace

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Create handles the API endpoint /namespaces (POST).
// It creates a namespace with the specified name.
func (oc Controller) Create(w http.ResponseWriter, r *http.Request) apierror.APIErrors {
	ctx := r.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return apierror.InternalError(err)
	}

	// map ~ json oject / Required key: name
	var parts map[string]string
	err = json.Unmarshal(bodyBytes, &parts)
	if err != nil {
		return apierror.BadRequest(err)
	}

	org, ok := parts["name"]
	if !ok {
		err := errors.New("name of namespace to create not found")
		return apierror.BadRequest(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}
	if exists {
		return apierror.OrgAlreadyKnown(org)
	}

	err = organizations.Create(r.Context(), cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = response.JSON(w, models.ResponseOK)
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
