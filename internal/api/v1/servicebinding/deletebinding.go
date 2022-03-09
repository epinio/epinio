package servicebinding

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

func DeleteBinding(ctx context.Context, cluster *kubernetes.Cluster, namespace, appName, serviceName, username string) apierror.APIErrors {

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	_, err = services.Lookup(ctx, cluster, namespace, serviceName)
	if err != nil && err.Error() == "service not found" {
		return apierror.ServiceIsNotKnown(serviceName)
	}
	if err != nil {
		return apierror.InternalError(err)
	}

	err = application.BoundServicesUnset(ctx, cluster, app.Meta, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app.Workload != nil {
		_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "", nil, nil)
		if apierr != nil {
			return apierr
		}
	}

	return nil
}
