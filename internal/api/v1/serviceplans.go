package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

type ServicePlansController struct {
}

func (spc ServicePlansController) Index(w http.ResponseWriter, r *http.Request) []APIError {
	params := httprouter.ParamsFromContext(r.Context())
	serviceClassName := params.ByName("serviceclass")

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	serviceClass, err := services.ClassLookup(cluster, serviceClassName)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	if serviceClass == nil {
		return []APIError{
			NewAPIError(fmt.Sprintf("ServiceClass '%s' does not exist", serviceClassName),
				"", http.StatusNotFound),
		}
	}
	servicePlans, err := serviceClass.ListPlans()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	js, err := json.Marshal(servicePlans)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return []APIError{}
}
