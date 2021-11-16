package namespace

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Match handles the API endpoint /namespaces/:pattern (GET)
// It returns a list of all Epinio-controlled namespaces matching the prefix pattern.
func (oc Controller) Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := tracelog.Logger(ctx)

	log.Info("match namespaces")
	defer log.Info("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Info("list namespaces")
	namespaces, err := namespaces.List(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Info("get namespace prefix")
	prefix := c.Param("pattern")

	log.Info("match prefix", "pattern", prefix)
	matches := []string{}
	for _, namespace := range namespaces {
		if strings.HasPrefix(namespace.Name, prefix) {
			matches = append(matches, namespace.Name)
		}
	}

	log.Info("deliver matches", "found", matches)

	response.OKReturn(c, models.NamespacesMatchResponse{
		Names: matches,
	})
	return nil
}
