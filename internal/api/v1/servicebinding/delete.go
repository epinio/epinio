package servicebinding

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint /orgs/:org/applications/:app/servicebindings/:service
// It removes the binding between the specified service and application
func (hc Controller) Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	org := c.Param("org")
	appName := c.Param("app")
	serviceName := c.Param("service")
	username := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}
	if !exists {
		return apierror.OrgIsNotKnown(org)
	}

	apiErr := DeleteBinding(ctx, cluster, org, appName, serviceName, username)
	if apiErr != nil {
		return apiErr
	}

	response.OK(c)
	return nil
}
