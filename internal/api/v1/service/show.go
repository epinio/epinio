package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Show handles the API end point /orgs/:org/services/:service
// It returns the detail information of the named service instance
func (sc Controller) Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	org := c.Param("org")
	serviceName := c.Param("service")

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

	service, err := services.Lookup(ctx, cluster, org, serviceName)
	if err != nil {
		if err.Error() == "service not found" {
			return apierror.ServiceIsNotKnown(serviceName)
		}
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	serviceDetails, err := service.Details(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	responseData := map[string]string{}
	for key, value := range serviceDetails {
		responseData[key] = value
	}

	err = response.JSON(c, models.ServiceShowResponse{
		Username: service.User(),
		Details:  responseData,
	})
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
