package service

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Match handles the API endpoint /namespace/:namespace/servicesmatches/:pattern (GET)
// It returns a list of all Epinio-controlled services matching the prefix pattern.
func (oc Controller) Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")

	log.Info("match services")
	defer log.Info("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	log.Info("list services")
	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	serviceList, err := kubeServiceClient.ListInNamespace(ctx, namespace)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	log.Info("get service prefix")
	prefix := c.Param("pattern")

	log.Info("match prefix", "pattern", prefix)
	matches := []string{}
	for _, service := range serviceList {
		if strings.HasPrefix(service.Meta.Name, prefix) {
			matches = append(matches, service.Meta.Name)
		}
	}

	log.Info("deliver matches", "found", matches)

	response.OKReturn(c, models.ServiceMatchResponse{
		Names: matches,
	})
	return nil
}
