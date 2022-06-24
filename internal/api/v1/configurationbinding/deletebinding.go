package configurationbinding

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

func DeleteBinding(ctx context.Context, cluster *kubernetes.Cluster, namespace, appName, configurationName, username string) apierror.APIErrors {

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.NewInternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	_, err = configurations.Lookup(ctx, cluster, namespace, configurationName)
	if err != nil && err.Error() == "configuration not found" {
		return apierror.ConfigurationIsNotKnown(configurationName)
	}
	if err != nil {
		return apierror.NewInternalError(err)
	}

	err = application.BoundConfigurationsUnset(ctx, cluster, app.Meta, configurationName)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	if app.Workload != nil {
		_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "", nil, nil)
		if apierr != nil {
			return apierr
		}
	}

	return nil
}
