package v1

import (
	"encoding/json"
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

	js, err := json.Marshal(serviceClasses)
	if err != nil {
		return InternalError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
