package configurationbinding

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
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

	apiErr := DeleteBinding(ctx, cluster, namespace, appName, configurationName, username)
	if apiErr != nil {
		return apiErr
	}

	response.OK(c)
	return nil
}
