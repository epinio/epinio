package servicebinding

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/julienschmidt/httprouter"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Delete handles the API endpoint /orgs/:org/applications/:app/servicebindings/:service
// It removes the binding between the specified service and application
func (hc Controller) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")
	serviceName := params.ByName("service")
	username := requestctx.User(ctx)

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

	apiErr := DeleteBinding(ctx, cluster, org, appName, serviceName, username)
	if apiErr != nil {
		return apiErr
	}

	err = response.JSON(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
