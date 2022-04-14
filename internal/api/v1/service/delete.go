package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

func (ctr Controller) Delete(c *gin.Context) apierror.APIErrors {
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

	err = kubeServiceClient.Delete(ctx, namespace, serviceName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return apierror.NewNotFoundError("service not found")
		}

		return apierror.InternalError(err)
	}

	response.OKReturn(c, nil)

	return nil
}
