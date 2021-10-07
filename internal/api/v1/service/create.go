package service

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/julienschmidt/httprouter"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Create handles the API end point /orgs/:org/services
// It creates the named service from its parameters
func (sc Controller) Create(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	username := requestctx.User(ctx)

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var createRequest models.ServiceCreateRequest
	err = json.Unmarshal(bodyBytes, &createRequest)
	if err != nil {
		return BadRequest(err)
	}

	if createRequest.Name == "" {
		return NewBadRequest("Cannot create service without a name")
	}

	if len(createRequest.Data) < 1 {
		return NewBadRequest("Cannot create service without data")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	// Verify that the requested name is not yet used by a different service.
	_, err = services.Lookup(ctx, cluster, org, createRequest.Name)
	if err == nil {
		// no error, service is found, conflict
		return ServiceAlreadyKnown(createRequest.Name)
	}
	if err != nil && err.Error() != "service not found" {
		// some internal error
		return InternalError(err)
	}
	// any error here is `service not found`, and we can continue

	// Create the new service. At last.
	_, err = services.CreateService(ctx, cluster, createRequest.Name, org, username, createRequest.Data)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = response.JSON(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
