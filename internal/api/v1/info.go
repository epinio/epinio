package v1

import (
	"encoding/json"
	"net/http"

	"github.com/epinio/epinio/internal/version"
)

type InfoController struct {
}

func (hc InfoController) Info(w http.ResponseWriter, r *http.Request) []APIError {
	info := struct {
		Version string
	}{
		Version: version.Version,
	}
	js, err := json.Marshal(info)
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
