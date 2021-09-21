package v1

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

// ServicePlansController represents all functionality of the API
// related to catalog service plans
type ServicePlansController struct {
}

// Index handles the API endpoint /serviceclasses/:serviceclass/serviceplans
// It returns a list of all service plans known to the catalog for
// the named service class.
func (spc ServicePlansController) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	serviceClassName := params.ByName("serviceclass")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	serviceClass, err := services.ClassLookup(ctx, cluster, serviceClassName)
	if err != nil {
		return InternalError(err)
	}

	if serviceClass == nil {
		return ServiceClassIsNotKnown(serviceClassName)
	}
	servicePlans, err := serviceClass.ListPlans(ctx)
	if err != nil {
		return InternalError(err)
	}

	err = jsonResponse(w, servicePlans)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
