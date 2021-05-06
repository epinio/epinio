package v1

import (
	"encoding/json"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/services"
)

type ServiceClassesController struct {
}

func (scc ServiceClassesController) Index(w http.ResponseWriter, r *http.Request) []APIError {
	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	serviceClasses, err := services.ListClasses(cluster)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	js, err := json.Marshal(serviceClasses)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)

	return []APIError{}
}
