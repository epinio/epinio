package namespace

import (
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Create handles the API endpoint /namespaces (POST).
// It creates a namespace with the specified name.
func (oc Controller) Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	var request models.NamespaceCreateRequest
	err := c.BindJSON(&request)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}
	namespaceName := request.Name

	if namespaceName == "" {
		return apierror.NewBadRequestError("name of namespace to create not found")
	}

	exists, err := oc.namespaceService.Exists(ctx, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if exists {
		return apierror.NamespaceAlreadyKnown(namespaceName)
	}

	err = oc.namespaceService.Create(ctx, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	user := requestctx.User(ctx)
	err = oc.authService.AddNamespaceToUser(ctx, user.Username, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}
