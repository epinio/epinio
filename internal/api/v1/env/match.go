package env

import (
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/julienschmidt/httprouter"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Match handles the API endpoint /orgs/:org/applications/:app/environment/:env/match/:pattern
// It receives the org, application name, plus a prefix and returns
// the names of all the environment associated with that application
// with prefix
func (hc Controller) Match(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	orgName := params.ByName("org")
	appName := params.ByName("app")
	prefix := params.ByName("pattern")

	log.Info("returning matching environment variable names",
		"org", orgName, "app", appName, "prefix", prefix)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, orgName)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return OrgIsNotKnown(orgName)
	}

	app := models.NewAppRef(appName, orgName)

	exists, err = application.Exists(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	// EnvList, with post-processing - selection of matches, and
	// projection to deliver only names

	environment, err := application.Environment(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	matches := []string{}
	for _, ev := range environment {
		if strings.HasPrefix(ev.Name, prefix) {
			matches = append(matches, ev.Name)
		}
	}

	err = response.JSON(w, models.EnvMatchResponse{Names: matches})
	if err != nil {
		return InternalError(err)
	}

	return nil
}
