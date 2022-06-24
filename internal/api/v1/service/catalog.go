package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (ctr Controller) Catalog(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	serviceList, err := kubeServiceClient.ListCatalogServices(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, serviceList)
	return nil
}

func (ctr Controller) CatalogShow(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	serviceName := c.Param("catalogservice")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	service, err := kubeServiceClient.GetCatalogService(ctx, serviceName)
	if err != nil {
		if k8sapierrors.IsNotFound(err) {
			return apierror.NewNotFoundError(errors.Wrap(err, "service instance doesn't exist"))
		}

		return apierror.InternalError(err)
	}

	response.OKReturn(c, service)
	return nil
}
