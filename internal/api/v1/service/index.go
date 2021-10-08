package service

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/julienschmidt/httprouter"
)

// Controller represents all functionality of the API related to services
type Controller struct {
}

// Index handles the API end point /orgs/:org/services
// It returns a list of all known service instances
func (sc Controller) Index(w http.ResponseWriter, r *http.Request) apierror.APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")

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

	orgServices, err := services.List(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	appsOf, err := servicesToApps(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	var responseData models.ServiceResponseList

	for _, service := range orgServices {
		var appNames []string

		for _, app := range appsOf[service.Name()] {
			appNames = append(appNames, app.Meta.Name)
		}
		responseData = append(responseData, models.ServiceResponse{
			Name:      service.Name(),
			BoundApps: appNames,
		})
	}

	err = response.JSON(w, responseData)
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
