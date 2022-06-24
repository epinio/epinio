package env

import (
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Match handles the API endpoint /namespaces/:namespace/applications/:app/environment/:env/match/:pattern
// It receives the namespace, application name, plus a prefix and returns
// the names of all the environment associated with that application
// with prefix
func (hc Controller) Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespaceName := c.Param("namespace")
	appName := c.Param("app")
	prefix := c.Param("pattern")

	log.Info("returning matching environment variable names",
		"namespace", namespaceName, "app", appName, "prefix", prefix)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	app := models.NewAppRef(appName, namespaceName)

	exists, err := application.Exists(ctx, cluster, app)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	if !exists {
		return apierror.AppIsNotKnown(appName)
	}

	// EnvList, with post-processing - selection of matches, and
	// projection to deliver only names

	environment, err := application.Environment(ctx, cluster, app)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	matches := []string{}
	for evName := range environment {
		if strings.HasPrefix(evName, prefix) {
			matches = append(matches, evName)
		}
	}
	sort.Strings(matches)

	response.OKReturn(c, models.EnvMatchResponse{
		Names: matches,
	})
	return nil
}
