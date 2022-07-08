package namespace

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"

	"github.com/gin-gonic/gin"
)

// Create handles the API endpoint /namespaces (POST).
// It creates a namespace with the specified name.
func (oc Controller) Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	var request models.NamespaceCreateRequest
	err = c.BindJSON(&request)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}
	namespaceName := request.Name

	if namespaceName == "" {
		return apierror.NewBadRequestError("name of namespace to create not found")
	}

	exists, err := namespaces.Exists(ctx, cluster, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if exists {
		return apierror.NamespaceAlreadyKnown(namespaceName)
	}

	err = namespaces.Create(ctx, cluster, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = addNamespaceToUser(ctx, namespaceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}

// addNamespaceToUser will add the namespace to the User namespaces
func addNamespaceToUser(ctx context.Context, namespace string) error {
	user := requestctx.User(ctx)

	authService, err := auth.NewAuthServiceFromContext(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating auth service")
	}

	err = authService.AddNamespaceToUser(ctx, user.Username, namespace)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error adding namespace [%s] to user [%s]", namespace, user.Username))
	}
	return nil
}
