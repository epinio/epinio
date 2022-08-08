package configuration

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Delete handles the API end point /namespaces/:namespace/configurations/:configuration (DELETE)
// It deletes the named configuration
func (sc Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	configurationName := c.Param("configuration")
	user := requestctx.User(ctx)

	var deleteRequest models.ConfigurationDeleteRequest
	err := c.BindJSON(&deleteRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	configuration, err := configurations.Lookup(ctx, cluster, namespace, configurationName)
	if err != nil && err.Error() == "configuration not found" {
		return apierror.ConfigurationIsNotKnown(configurationName)
	}
	if err != nil {
		return apierror.InternalError(err)
	}

	// Verify that the configuration is unbound. IOW not bound to any application.
	// If it is, and automatic unbind was requested, do that.
	// Without automatic unbind such applications are reported as error.

	boundAppNames, err := application.BoundAppsNamesFor(ctx, cluster, namespace, configurationName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if len(boundAppNames) > 0 {
		if !deleteRequest.Unbind {
			return apierror.NewBadRequestError("bound applications exist").WithDetails(strings.Join(boundAppNames, ","))
		}

		for _, appName := range boundAppNames {
			apiErr := configurationbinding.DeleteBinding(ctx, cluster, namespace, appName, configurationName, user)
			if apiErr != nil {
				return apiErr
			}
		}
	}

	// Everything looks to be ok. Delete.

	err = configuration.Delete(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, models.ConfigurationDeleteResponse{
		BoundApps: boundAppNames,
	})
	return nil
}
