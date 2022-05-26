package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
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

	appsOf, err := application.ServicesBoundAppsNames(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}

	filteredServices := auth.FilterResources(user, serviceList)

	response.OKReturn(c, extendWithBoundApps(filteredServices, appsOf))
	return nil
}

func extendWithBoundApps(services models.ServiceList, appsOf map[string][]string) models.ServiceList {
	theServices := models.ServiceList{}
	for _, service := range services {
		key := application.ServiceKey(service.Meta.Name, service.Meta.Namespace)

		service.BoundApps = appsOf[key]
		theServices = append(theServices, service)
	}
	return theServices
}
