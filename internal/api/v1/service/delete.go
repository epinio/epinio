package service

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/api/v1/servicebinding"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/julienschmidt/httprouter"
)

// Delete handles the API end point /orgs/:org/services/:service (DELETE)
// It deletes the named service
func (sc Controller) Delete(w http.ResponseWriter, r *http.Request) apierror.APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	serviceName := params.ByName("service")
	username := requestctx.User(ctx)

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return apierror.InternalError(err)
	}

	var deleteRequest models.ServiceDeleteRequest
	err = json.Unmarshal(bodyBytes, &deleteRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

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
	if err != nil && err.Error() == "service not found" {
		return apierror.ServiceIsNotKnown(serviceName)
	}
	if err != nil {
		return apierror.InternalError(err)
	}

	// Verify that the service is unbound. IOW not bound to any application.
	// If it is, and automatic unbind was requested, do that.
	// Without automatic unbind such applications are reported as error.

	boundAppNames := []string{}
	appsOf, err := servicesToApps(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}
	if boundApps, found := appsOf[service.Name()]; found {
		for _, app := range boundApps {
			boundAppNames = append(boundAppNames, app.Meta.Name)
		}

		if !deleteRequest.Unbind {
			return apierror.NewBadRequest("bound applications exist", strings.Join(boundAppNames, ","))
		}

		for _, app := range boundApps {
			apiErr := servicebinding.DeleteBinding(ctx, cluster, org, app.Meta.Name, serviceName, username)
			if apiErr != nil {
				return apiErr
			}
		}
	}

	// Everything looks to be ok. Delete.

	err = service.Delete(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = response.JSON(w, models.ServiceDeleteResponse{BoundApps: boundAppNames})
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}
