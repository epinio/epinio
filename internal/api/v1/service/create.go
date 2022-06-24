package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/services"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

func (ctr Controller) Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	var createRequest models.ServiceCreateRequest
	err := c.BindJSON(&createRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	catalogService, err := kubeServiceClient.GetCatalogService(ctx, createRequest.CatalogService)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return apierror.NewBadRequest(err.Error()).WithDetailsf("Catalog service %s not found", createRequest.CatalogService)
		}
		return apierror.InternalError(err)
	}

	err = kubeServiceClient.Create(ctx, namespace, createRequest.Name, *catalogService)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}
