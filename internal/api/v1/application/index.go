package application

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"

	"github.com/julienschmidt/httprouter"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Controller represents all functionality of the API related to applications
type Controller struct {
}

// Index handles the API endpoint GET /namespaces/:org/applications
// It lists all the known applications in the specified namespace, with and without workload.
func (hc Controller) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")

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

	apps, err := application.List(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	err = response.JSON(w, apps)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
