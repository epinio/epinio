package env

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// EnvShow handles the API endpoint /namespaces/:namespace/applications/:app/environment/:env
// It receives the namespace, application name, var name, and returns
// the variable's value in the application's environment.
func (hc Controller) Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespaceName := c.Param("namespace")
	appName := c.Param("app")
	varName := c.Param("env")

	log.Info("processing environment variable request",
		"namespace", namespaceName, "app", appName, "var", varName)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	app := models.NewAppRef(appName, namespaceName)

	exists, err := application.Exists(ctx, cluster, app)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	if !exists {
		return apierror.AppIsNotKnown(appName)
	}

	// EnvList, with post-processing - select specific value

	environment, err := application.Environment(ctx, cluster, app)
	if err != nil {
		return apierror.NewInternalError(err)
	}

	match := models.EnvVariable{}

	value, ok := environment[varName]
	if ok {
		match.Name = varName
		match.Value = value
	}
	// Not found: Returns an empty object.

	response.OKReturn(c, match)
	return nil
}
