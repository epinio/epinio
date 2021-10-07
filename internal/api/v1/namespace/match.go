package namespace

import (
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/julienschmidt/httprouter"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Match handles the API endpoint /namespaces/:pattern (GET)
// It returns a list of all Epinio-controlled namespaces matching the prefix pattern.
func (oc Controller) Match(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	log.Info("match namespaces")
	defer log.Info("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	log.Info("list namespaces")
	namespaces, err := organizations.List(ctx, cluster)
	if err != nil {
		return InternalError(err)
	}

	log.Info("get namespace prefix")
	params := httprouter.ParamsFromContext(ctx)
	prefix := params.ByName("pattern")

	log.Info("match prefix", "pattern", prefix)
	matches := []string{}
	for _, namespace := range namespaces {
		if strings.HasPrefix(namespace.Name, prefix) {
			matches = append(matches, namespace.Name)
		}
	}

	log.Info("deliver matches", "found", matches)

	err = response.JSON(w, models.NamespacesMatchResponse{Names: matches})
	if err != nil {
		return InternalError(err)
	}

	return nil
}
