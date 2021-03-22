package v1

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/suse/carrier/internal/application"
	"github.com/suse/carrier/internal/cli/clients"
)

type ApplicationsController struct {
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	client, cleanup, err := clients.NewCarrierClient(nil)
	if handleError(w, err, 500) {
		return
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	apps, err := application.List(client.KubeClient, client.GiteaClient, org)
	if handleError(w, err, 500) {
		return
	}

	js, err := json.Marshal(apps)
	if handleError(w, err, 500) {
		return
	}
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
