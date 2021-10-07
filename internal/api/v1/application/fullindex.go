package application

import (
	"encoding/json"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Index handles the API endpoint GET /applications
// It lists all the known applications in all namespaces, with and without workload.
func (hc Controller) FullIndex(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	allApps, err := application.List(ctx, cluster, "")
	if err != nil {
		return InternalError(err)
	}

	js, err := json.Marshal(allApps)
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
