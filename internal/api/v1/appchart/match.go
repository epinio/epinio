package appchart

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Match handles the API endpoint /appchartsmatch/:pattern (GET)
// It returns a list of all Epinio-controlled appcharts matching the prefix pattern.
func (oc Controller) Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	log.Info("match appcharts")
	defer log.Info("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Info("list appcharts")
	appcharts, err := appchart.List(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Info("get appchart prefix")
	prefix := c.Param("pattern")

	log.Info("match prefix", "pattern", prefix)
	matches := []string{}
	for _, appchart := range appcharts {
		if strings.HasPrefix(appchart.Name, prefix) {
			matches = append(matches, appchart.Name)
		}
	}

	log.Info("deliver matches", "found", matches)

	response.OKReturn(c, models.ChartMatchResponse{
		Names: matches,
	})
	return nil
}
