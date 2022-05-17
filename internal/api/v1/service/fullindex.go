package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

func (ctr Controller) FullIndex(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	user := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	serviceList, err := kubeServiceClient.ListAll(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	resp := models.ServiceListResponse{
		Services: filterServices(user, serviceList),
	}

	response.OKReturn(c, resp)
	return nil
}

func filterServices(user auth.User, services []*models.Service) []*models.Service {
	if user.Role == "admin" {
		return services
	}

	namespacesMap := make(map[string]struct{})
	for _, ns := range user.Namespaces {
		namespacesMap[ns] = struct{}{}
	}

	filteredServices := []*models.Service{}
	for _, service := range services {
		if _, allowed := namespacesMap[service.Namespace]; allowed {
			filteredServices = append(filteredServices, service)
		}
	}

	return filteredServices
}
