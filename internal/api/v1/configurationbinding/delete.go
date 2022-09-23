package configurationbinding

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint /namespaces/:namespace/applications/:app/configurationbindings/:configuration
// It removes the binding between the specified configuration and application
func (hc Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	configurationName := c.Param("configuration")
	username := requestctx.User(ctx).Username

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	config, err := configurations.Lookup(ctx, cluster, namespace, configurationName)
	if err != nil && err.Error() == "configuration not found" {
		return apierror.ConfigurationIsNotKnown(configurationName)
	}
	if err != nil {
		return apierror.InternalError(err)
	}

	if config.Origin != "" {
		return apierror.NewBadRequestErrorf("Configuration belongs to service '%s', use service requests",
			config.Origin)
	}

	apiErr := DeleteBinding(ctx, cluster, namespace, appName, username, []string{configurationName})
	if apiErr != nil {
		return apiErr
	}

	response.OK(c)
	return nil
}
