package namespace

import (
	"errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

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
		return apierror.BadRequest(err)
	}

	if request.Name == "" {
		err := errors.New("name of namespace to create not found")
		return apierror.BadRequest(err)
	}

	exists, err := namespaces.Exists(ctx, cluster, request.Name)
	if err != nil {
		return apierror.InternalError(err)
	}
	if exists {
		// TODO: Don't retry when this happens (on the cli)
		return apierror.NamespaceAlreadyKnown(request.Name)
	}

	err = namespaces.Create(ctx, cluster, request.Name)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}
