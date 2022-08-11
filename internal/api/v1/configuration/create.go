package configuration

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Create handles the API end point /namespaces/:namespace/configurations
// It creates the named configuration from its parameters
func (sc Controller) Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	username := requestctx.User(ctx).Username

	var createRequest models.ConfigurationCreateRequest
	err := c.BindJSON(&createRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	if createRequest.Name == "" {
		return apierror.NewBadRequestError("cannot create configuration without a name")
	}

	if len(createRequest.Data) < 1 {
		return apierror.NewBadRequestError("cannot create configuration without data")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Verify that the requested name is not yet used by a different configuration.
	_, err = configurations.Lookup(ctx, cluster, namespace, createRequest.Name)
	if err == nil {
		// no error, configuration is found, conflict
		return apierror.ConfigurationAlreadyKnown(createRequest.Name)
	}
	if err != nil && err.Error() != "configuration not found" {
		// some internal error
		return apierror.InternalError(err)
	}
	// any error here is `configuration not found`, and we can continue

	// Create the new configuration. At last.
	_, err = configurations.CreateConfiguration(ctx, cluster, createRequest.Name, namespace, username, createRequest.Data)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}
