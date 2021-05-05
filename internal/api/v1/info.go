package v1

import (
	"encoding/json"
	"net/http"

	"github.com/epinio/epinio/internal/version"
)

type InfoController struct {
}

func (hc InfoController) Info(w http.ResponseWriter, r *http.Request) APIErrors {
	info := struct {
		Version string
	}{
		Version: version.Version,
	}
	js, err := json.Marshal(info)
	if err != nil {
		return APIErrors{InternalError(err)}
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	return nil
}
