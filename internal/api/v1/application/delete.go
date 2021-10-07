package application

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/julienschmidt/httprouter"
)

// Delete handles the API endpoint DELETE /namespaces/:org/applications/:app
// It removes the named application
func (hc Controller) Delete(w http.ResponseWriter, r *http.Request) apierror.APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.OrgIsNotKnown(org)
	}

	app := models.NewAppRef(appName, org)

	found, err := application.Exists(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}
	if !found {
		return apierror.AppIsNotKnown(appName)
	}

	services, err := application.BoundServiceNames(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	resp := models.ApplicationDeleteResponse{UnboundServices: services}

	err = application.Delete(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = response.JSON(w, resp)
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
