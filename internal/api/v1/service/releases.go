package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	helmrelease "helm.sh/helm/v3/pkg/release"
)

func (ctr Controller) Bind(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	serviceReleaseName := c.Param("servicereleasename")

	var bindRequest models.ServiceReleaseBindRequest
	err := c.BindJSON(&bindRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	app, err := application.Lookup(ctx, cluster, namespace, bindRequest.AppName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(bindRequest.AppName)
	}

	client, err := getHelmClient(cluster.RestConfig, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	release, err := client.GetRelease(serviceReleaseName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if release.Info.Status != helmrelease.StatusDeployed {
		return apierror.InternalError(err)
	}

	// label the secrets
	configurationSecrets, err := configurations.LabelReleaseSecrets(ctx, cluster, namespace, serviceReleaseName)
	if err != nil {
		return apierror.InternalError(err)
	}

	configurationNames := []string{}
	for _, secret := range configurationSecrets {
		configurationNames = append(configurationNames, secret.Name)
	}

	_, errors := configurationbinding.CreateConfigurationBinding(
		ctx, cluster, namespace, *app, configurationNames,
	)
	if len(errors.Errors()) > 0 {
		return apierror.NewMultiError(errors.Errors())
	}

	response.OK(c)
	return nil
}
