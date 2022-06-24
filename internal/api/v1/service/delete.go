package service

import (
	"fmt"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// Delete handles the API end point /namespaces/:namespace/services/:service (DELETE)
// It deletes the named service
func (ctr Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).WithName("Delete")
	namespace := c.Param("namespace")
	serviceName := c.Param("service")
	// username := requestctx.User(ctx).Username

	var deleteRequest models.ServiceDeleteRequest
	err := c.BindJSON(&deleteRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	apiErr := ValidateService(ctx, cluster, logger, namespace, serviceName)
	if apiErr != nil {
		return apiErr
	}

	// A service has one or more associated secrets containing its attributes.
	// Binding turned these secrets into configurations and bound them to the
	// application.  Unbinding simply unbound them.  We may think that this means that
	// we only have to look for the first configuration to determine what apps the
	// service is bound to. Not so. With the secrets visible as configurations an
	// adventurous user may have unbound them in part, and left in part. So, check
	// everything, and then de-duplicate.

	boundAppNames := []string{}

	serviceConfigurations, err := configurations.ForService(ctx, cluster, namespace, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info(fmt.Sprintf("configurationSecrets found %+v\n", serviceConfigurations))

	for _, secret := range serviceConfigurations {
		bound, err := application.BoundAppsNamesFor(ctx, cluster, namespace, secret.Name)
		if err != nil {
			return apierror.InternalError(err)
		}

		boundAppNames = append(boundAppNames, bound...)
	}

	boundAppNames = helpers.UniqueStrings(boundAppNames)

	// Verify that the service is unbound. IOW not bound to any application.
	// If it is, and automatic unbind was requested, do that.
	// Without automatic unbind such applications are reported as error.

	if len(boundAppNames) > 0 {
		if !deleteRequest.Unbind {
			return apierror.NewBadRequest("bound applications exist", strings.Join(boundAppNames, ","))
		}

		username := requestctx.User(ctx).Username

		// Unbind all the services' configurations from the found applications.
		for _, appName := range boundAppNames {
			apiErr := UnbindService(ctx, cluster, logger, namespace, serviceName, appName, username, serviceConfigurations)
			if apiErr != nil {
				return apiErr
			}
		}
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = kubeServiceClient.Delete(ctx, namespace, serviceName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return apierror.NewNotFoundError(errors.Wrap(err, "service not found"))
		}

		return apierror.InternalError(err)
	}

	response.OKReturn(c, models.ServiceDeleteResponse{
		BoundApps: boundAppNames,
	})
	return nil
}
