package v1

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

type ServicesController struct {
}

func (sc ServicesController) Show(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	serviceName := params.ByName("service")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	service, err := services.Lookup(cluster, org, serviceName)
	if err != nil {
		if err.Error() == "service not found" {
			http.Error(w, fmt.Sprintf("Service '%s' does not exist", serviceName),
				http.StatusNotFound)
			return
		}
		if handleError(w, err, http.StatusInternalServerError) {
			return
		}
	}

	status, err := service.Status()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	serviceDetails, err := service.Details()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	responseData := map[string]string{
		"Status": status,
	}
	for key, value := range serviceDetails {
		responseData[key] = value
	}

	js, err := json.Marshal(responseData)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (sc ServicesController) Index(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	orgServices, err := services.List(cluster, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	appsOf, err := servicesToApps(cluster, gitea, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
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
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (sc ServicesController) CreateCustom(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	var createRequest models.CustomCreateRequest
	err = json.Unmarshal(bodyBytes, &createRequest)
	if handleError(w, err, http.StatusBadRequest) {
		return
	}

	if createRequest.Name == "" {
		http.Error(w, fmt.Sprintf("Cannot create custom service without a name"),
			http.StatusBadRequest)
		return
	}

	if len(createRequest.Data) < 1 {
		http.Error(w, fmt.Sprintf("Cannot create custom service without data"),
			http.StatusBadRequest)
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	// Verify that the requested name is not yet used by a different service.
	_, err = services.Lookup(cluster, org, createRequest.Name)
	if err == nil {
		http.Error(w, fmt.Sprintf("Service '%s' already exists", createRequest.Name),
			http.StatusConflict)
		return
	}

	// Create the new service. At last.
	_, err = services.CreateCustomService(cluster, createRequest.Name, org, createRequest.Data)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte{})
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (sc ServicesController) Create(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	var createRequest models.CatalogCreateRequest
	err = json.Unmarshal(bodyBytes, &createRequest)
	if handleError(w, err, http.StatusBadRequest) {
		return
	}

	if createRequest.Name == "" {
		http.Error(w, fmt.Sprintf("Cannot create service without a name"),
			http.StatusBadRequest)
		return
	}

	if createRequest.Class == "" {
		http.Error(w, fmt.Sprintf("Cannot create service without a service class"),
			http.StatusBadRequest)
		return
	}

	if createRequest.Plan == "" {
		http.Error(w, fmt.Sprintf("Cannot create service without a service plan"),
			http.StatusBadRequest)
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	// Verify that the requested name is not yet used by a different service.
	_, err = services.Lookup(cluster, org, createRequest.Name)
	if err == nil {
		http.Error(w, fmt.Sprintf("Service '%s' already exists", createRequest.Name),
			http.StatusConflict)
		return
	}

	// Verify that the requested class is supported
	serviceClass, err := services.ClassLookup(cluster, createRequest.Class)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if serviceClass == nil {
		http.Error(w, fmt.Sprintf("Service class '%s' does not exist", createRequest.Class),
			http.StatusNotFound)
		return
	}

	// Verify that the requested plan is supported by the class.
	servicePlan, err := serviceClass.LookupPlan(createRequest.Plan)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	if servicePlan == nil {
		http.Error(w, fmt.Sprintf("Service plan '%s' does not exist for class '%s'",
			createRequest.Plan, createRequest.Class),
			http.StatusNotFound)
		return
	}

	// Create the new service. At last.
	service, err := services.CreateCatalogService(cluster, createRequest.Name, org,
		createRequest.Class, createRequest.Plan, createRequest.Data)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	// Wait for service to be fully provisioned, if requested
	if createRequest.WaitForProvision {
		err := service.WaitForProvision()
		if handleError(w, err, http.StatusInternalServerError) {
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte{})
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func (sc ServicesController) Delete(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	serviceName := params.ByName("service")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	var deleteRequest models.DeleteRequest
	err = json.Unmarshal(bodyBytes, &deleteRequest)
	if handleError(w, err, http.StatusBadRequest) {
		return
	}

	cluster, err := kubernetes.GetCluster()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	gitea, err := clients.GetGiteaClient()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	exists, err := gitea.OrgExists(org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("Organization '%s' does not exist", org),
			http.StatusNotFound)
		return
	}

	service, err := services.Lookup(cluster, org, serviceName)
	if err != nil && err.Error() == "service not found" {
		http.Error(w, fmt.Sprintf("service '%s' not found", serviceName),
			http.StatusNotFound)
		return
	}

	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	// Verify that the service is unbound. IOW not bound to any application.
	// If it is, and automatic unbind was requested, do that.
	// Without automatic unbind such applications are reported as error.

	deleteResponse := models.DeleteResponse{}

	appsOf, err := servicesToApps(cluster, gitea, org)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
	if boundApps, found := appsOf[service.Name()]; found {
		for _, app := range boundApps {
			deleteResponse.BoundApps = append(deleteResponse.BoundApps, app.Name)
		}

		if !deleteRequest.Unbind {
			js, err := json.Marshal(deleteResponse)
			if handleError(w, err, http.StatusInternalServerError) {
				return
			}

			w.Header().Set("Content-Type", "application/json")
			http.Error(w, string(js), http.StatusBadRequest)
			return
		}

		for _, app := range boundApps {
			err = app.Unbind(service)
			if handleError(w, err, http.StatusInternalServerError) {
				return
			}
		}
	}

	// Everything looks to be ok. Delete.

	err = service.Delete()
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	js, err := json.Marshal(deleteResponse)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}
}

func servicesToApps(cluster *kubernetes.Cluster, gitea *clients.GiteaClient, org string) (map[string]application.ApplicationList, error) {
	// Determine apps bound to services
	// (inversion of services bound to apps)
	// Literally query apps in the org for their services and invert.

	var appsOf = map[string]application.ApplicationList{}

	apps, err := application.List(cluster, gitea.Client, org)
	if err != nil {
		return nil, err
	}

	for _, app := range apps {
		bound, err := app.Services()
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
