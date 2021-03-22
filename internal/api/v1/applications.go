package v1

import (
	"encoding/json"
	"net/http"
)

type ApplicationsController struct {
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) {
	applications := map[string]string{
		"application1": "running",
		"application2": "running",
	}

	js, err := json.Marshal(applications)
	handleError(w, err, 500)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// Write the error to the response writer and return  true if there was an error
func handleError(w http.ResponseWriter, err error, code int) bool {
	if err != nil {
		http.Error(w, err.Error(), 500)
		return true
	}
	return false
}
