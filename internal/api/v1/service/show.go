package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

func (ctr Controller) Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	serviceName := c.Param("service")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	if err := ctr.validateNamespace(ctx, cluster, namespace); err != nil {
		return err
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	srv, err := kubeServiceClient.Get(ctx, namespace, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if srv == nil {
		return apierror.ServiceIsNotKnown(serviceName)
	}

	appNames, err := application.ServicesBoundAppsNamesFor(ctx, cluster, namespace, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	srv.BoundApps = appNames

	resp := models.ServiceShowResponse{
		Service: srv,
	}

	response.OKReturn(c, resp)

	return nil
}
