package service

import (
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	helmrelease "helm.sh/helm/v3/pkg/release"
)

func (ctr Controller) Bind(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).WithName("Bind")

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

	logger.Info("looking for application")
	app, err := application.Lookup(ctx, cluster, namespace, bindRequest.AppName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(bindRequest.AppName)
	}

	logger.Info("getting helm client")

	client, err := getHelmClient(cluster.RestConfig, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("looking for release")
	release, err := client.GetRelease(serviceReleaseName)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info(fmt.Sprintf("release found %+v\n", release))
	if release.Info.Status != helmrelease.StatusDeployed {
		return apierror.InternalError(err)
	}

	// label the secrets
	logger.Info("looking for secrets to label")

	configurationSecrets, err := configurations.LabelReleaseSecrets(ctx, cluster, namespace, serviceReleaseName)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info(fmt.Sprintf("configurationSecrets found %+v\n", configurationSecrets))

	configurationNames := []string{}
	for _, secret := range configurationSecrets {
		configurationNames = append(configurationNames, secret.Name)
	}

	logger.Info("binding service configuration")

	_, errors := configurationbinding.CreateConfigurationBinding(
		ctx, cluster, namespace, *app, configurationNames,
	)

	if errors != nil {
		return apierror.NewMultiError(errors.Errors())
	}

	response.OK(c)
	return nil
}
