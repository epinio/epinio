package v1

import (
	"encoding/json"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/services"
)

type ServiceClassesController struct {
}

func (scc ServiceClassesController) Index(w http.ResponseWriter, r *http.Request) {
	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	serviceClasses, err := services.ListClasses(cluster)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	js, err := json.Marshal(serviceClasses)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
