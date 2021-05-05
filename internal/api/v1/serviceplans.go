package v1

import (
	"encoding/json"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

type ServicePlansController struct {
}

func (spc ServicePlansController) Index(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	serviceClassName := params.ByName("serviceclass")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	serviceClass, err := services.ClassLookup(cluster, serviceClassName)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if serviceClass == nil {
		err := errors.Errorf("ServiceClass '%s' does not exist", serviceClassName)
		handleError(w, err, http.StatusNotFound)
		return
	}
	servicePlans, err := serviceClass.ListPlans()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	js, err := json.Marshal(servicePlans)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}
