package v1

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

type ServicesController struct {
}

func (sc ServicesController) Show(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	serviceName := params.ByName("service")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	service, err := services.Lookup(ctx, cluster, org, serviceName)
	if err != nil {
		if err.Error() == "service not found" {
			return ServiceIsNotKnown(serviceName)
		}
		if err != nil {
			return InternalError(err)
		}
	}

	status, err := service.Status(ctx)
	if err != nil {
		return InternalError(err)
	}
	serviceDetails, err := service.Details(ctx)
	if err != nil {
		return InternalError(err)
	}

	responseData := map[string]string{
		"Status": status,
	}
	for key, value := range serviceDetails {
		responseData[key] = value
	}

	js, err := json.Marshal(responseData)
	if err != nil {
		return InternalError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func (sc ServicesController) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	orgServices, err := services.List(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	appsOf, err := servicesToApps(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	var responseData models.ServiceResponseList

	for _, service := range orgServices {
		var appNames []string

		for _, app := range appsOf[service.Name()] {
			appNames = append(appNames, app.Name)
		}
		responseData = append(responseData, models.ServiceResponse{
			Name:      service.Name(),
			BoundApps: appNames,
		})
	}

	js, err := json.Marshal(responseData)
	if err != nil {
		return InternalError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func (sc ServicesController) CreateCustom(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var createRequest models.CustomCreateRequest
	err = json.Unmarshal(bodyBytes, &createRequest)
	if err != nil {
		return BadRequest(err)
	}

	if createRequest.Name == "" {
		return NewBadRequest("Cannot create custom service without a name")
	}

	if len(createRequest.Data) < 1 {
		return NewBadRequest("Cannot create custom service without data")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	// Verify that the requested name is not yet used by a different service.
	_, err = services.Lookup(ctx, cluster, org, createRequest.Name)
	if err == nil {
		// no error, service is found, conflict
		return ServiceAlreadyKnown(createRequest.Name)
	}
	if err != nil && err.Error() != "service not found" {
		// some internal error
		return InternalError(err)
	}
	// any error here is `service not found`, and we can continue

	// Create the new service. At last.
	_, err = services.CreateCustomService(ctx, cluster, createRequest.Name, org, createRequest.Data)
	if err != nil {
		return InternalError(err)
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte{})
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func (sc ServicesController) Create(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var createRequest models.CatalogCreateRequest
	err = json.Unmarshal(bodyBytes, &createRequest)
	if err != nil {
		return BadRequest(err)
	}

	if createRequest.Name == "" {
		return NewBadRequest("Cannot create service without a name")
	}

	if createRequest.Class == "" {
		return NewBadRequest("Cannot create service without a service class")
	}

	if createRequest.Plan == "" {
		return NewBadRequest("Cannot create service without a service plan")
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	// Verify that the requested name is not yet used by a different service.
	_, err = services.Lookup(ctx, cluster, org, createRequest.Name)
	if err == nil {
		// no error, service is found, conflict
		return ServiceAlreadyKnown(createRequest.Name)
	}
	if err != nil && err.Error() != "service not found" {
		// some internal error
		return InternalError(err)
	}
	// any error here is `service not found`, and we can continue

	// Verify that the requested class is supported
	serviceClass, err := services.ClassLookup(ctx, cluster, createRequest.Class)
	if err != nil {
		return InternalError(err)
	}
	if serviceClass == nil {
		return ServiceClassIsNotKnown(createRequest.Class)
	}

	// Verify that the requested plan is supported by the class.
	servicePlan, err := serviceClass.LookupPlan(ctx, createRequest.Plan)
	if err != nil {
		return InternalError(err)
	}

	if servicePlan == nil {
		return ServicePlanIsNotKnown(createRequest.Plan, createRequest.Class)
	}

	// Create the new service. At last.
	service, err := services.CreateCatalogService(ctx, cluster, createRequest.Name, org,
		createRequest.Class, createRequest.Plan, createRequest.Data)
	if err != nil {
		return InternalError(err)
	}

	// Wait for service to be fully provisioned, if requested
	if createRequest.WaitForProvision {
		err := service.WaitForProvision(ctx)
		if err != nil {
			return InternalError(err)
		}
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte{})
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func (sc ServicesController) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	serviceName := params.ByName("service")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var deleteRequest models.DeleteRequest
	err = json.Unmarshal(bodyBytes, &deleteRequest)
	if err != nil {
		return BadRequest(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if !exists {
		return OrgIsNotKnown(org)
	}

	service, err := services.Lookup(ctx, cluster, org, serviceName)
	if err != nil && err.Error() == "service not found" {
		return ServiceIsNotKnown(serviceName)
	}
	if err != nil {
		return InternalError(err)
	}

	// Verify that the service is unbound. IOW not bound to any application.
	// If it is, and automatic unbind was requested, do that.
	// Without automatic unbind such applications are reported as error.

	boundAppNames := []string{}
	appsOf, err := servicesToApps(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}
	if boundApps, found := appsOf[service.Name()]; found {
		for _, app := range boundApps {
			boundAppNames = append(boundAppNames, app.Name)
		}

		if !deleteRequest.Unbind {
			return NewBadRequest("bound applications exist", strings.Join(boundAppNames, ","))
		}

		for _, app := range boundApps {
			err = app.Unbind(ctx, service)
			if err != nil {
				return InternalError(err)
			}
		}
	}

	// Everything looks to be ok. Delete.

	err = service.Delete(ctx)
	if err != nil {
		return InternalError(err)
	}

	js, err := json.Marshal(models.DeleteResponse{BoundApps: boundAppNames})
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func servicesToApps(ctx context.Context, cluster *kubernetes.Cluster, org string) (map[string]application.ApplicationList, error) {
	// Determine apps bound to services
	// (inversion of services bound to apps)
	// Literally query apps in the org for their services and invert.

	var appsOf = map[string]application.ApplicationList{}

	apps, err := application.List(ctx, cluster, org)
	if err != nil {
		return nil, err
	}

	for _, app := range apps {
		bound, err := app.Services(ctx)
		if err != nil {
			return nil, err
		}
		for _, bonded := range bound {
			bname := bonded.Name()
			if theapps, found := appsOf[bname]; found {
				appsOf[bname] = append(theapps, app)
			} else {
				appsOf[bname] = application.ApplicationList{app}
			}
		}
	}

	return appsOf, nil
}
