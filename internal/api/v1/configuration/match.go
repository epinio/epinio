package configuration

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Match handles the API endpoint /namespace/:namespace/configurationsmatches/:pattern (GET)
// It returns a list of all Epinio-controlled configurations matching the prefix pattern.
func (oc Controller) Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")

	log.Info("match configurations")
	defer log.Info("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	log.Info("list configurations")

	configurationList, err := configurations.List(ctx, cluster, namespace)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	log.Info("get configuration prefix")
	prefix := c.Param("pattern")

	log.Info("match prefix", "pattern", prefix)
	matches := []string{}
	for _, config := range configurationList {
		if strings.HasPrefix(config.Name, prefix) {
			matches = append(matches, config.Name)
		}
	}

	log.Info("deliver matches", "found", matches)

	response.OKReturn(c, models.ConfigurationMatchResponse{
		Names: matches,
	})
	return nil
}
