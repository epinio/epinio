package application

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Match handles the API endpoint /namespace/:namespace/appsmatches/:pattern (GET)
// It returns a list of all Epinio-controlled applications matching the prefix pattern.
func (oc Controller) Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")

	log.Info("match applications")
	defer log.Info("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	log.Info("list applications")

	apps, err := application.List(ctx, cluster, namespace)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	log.Info("get application prefix")
	prefix := c.Param("pattern")

	log.Info("match prefix", "pattern", prefix)
	matches := []string{}
	for _, app := range apps {
		if strings.HasPrefix(app.Meta.Name, prefix) {
			matches = append(matches, app.Meta.Name)
		}
	}

	log.Info("deliver matches", "found", matches)

	response.OKReturn(c, models.AppMatchResponse{
		Names: matches,
	})
	return nil
}
