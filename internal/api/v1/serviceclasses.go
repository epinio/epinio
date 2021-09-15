package v1

import (
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/services"
)

// ServiceClassesController represents all functionality of the API
// related to catalog service classes
type ServiceClassesController struct {
}

// Index handles the API endpoint /serviceclasses. It returns a list
// of all service classes known to the catalog.
func (scc ServiceClassesController) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	serviceClasses, err := services.ListClasses(ctx, cluster)
	if err != nil {
		return InternalError(err)
	}

	err = jsonResponse(w, serviceClasses)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
