package service

import (
	"errors"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/names"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"

	helmrelease "helm.sh/helm/v3/pkg/release"
)

// Bind handles the API endpoint /namespaces/:namespace/services/:service/bind (POST)
// It creates a binding between the specified service and application
func (ctr Controller) Bind(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).WithName("Bind")

	namespace := c.Param("namespace")
	serviceName := c.Param("service")

	var bindRequest models.ServiceBindRequest
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

	client, err := helm.GetHelmClient(cluster.RestConfig, logger, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("looking for service")
	releaseName := names.ServiceHelmChartName(serviceName, namespace)
	srv, err := client.GetRelease(releaseName)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			return apierror.NewNotFoundError(fmt.Sprintf("%s - %s", err.Error(), releaseName))
		}
		return apierror.InternalError(err)
	}

	logger.Info(fmt.Sprintf("service found %+v\n", serviceName))
	if srv.Info.Status != helmrelease.StatusDeployed {
		return apierror.InternalError(err)
	}

	// A service has one or more associated secrets containing its attributes. Adding
	// a specific set of labels turns these secrets into valid epinio
	// configurations. These configurations are then bound to the application.

	logger.Info("looking for secrets to label")

	configurationSecrets, err := configurations.LabelServiceSecrets(ctx, cluster, namespace, serviceName)
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
