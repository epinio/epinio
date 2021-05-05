package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/julienschmidt/httprouter"
)

type ApplicationsController struct {
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	if !exists {
		return APIErrors{OrgIsNotKnown(org)}
	}

	apps, err := application.List(cluster, org)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	js, err := json.Marshal(apps)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	return nil
}

func (hc ApplicationsController) Show(w http.ResponseWriter, r *http.Request) APIErrors {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	if !exists {
		return APIErrors{OrgIsNotKnown(org)}
	}

	app, err := application.Lookup(cluster, org, appName)
	if err != nil {
		return APIErrors{InternalError(err)}
	}
	if app == nil {
		return APIErrors{AppIsNotKnown(appName)}
	}

	js, err := json.Marshal(app)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	return nil
}

func (hc ApplicationsController) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	gitea, err := gitea.New()
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	exists, err := organizations.Exists(cluster, org)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	if !exists {
		return APIErrors{OrgIsNotKnown(org)}
	}

	app, err := application.Lookup(cluster, org, appName)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	if app == nil {
		return APIErrors{AppIsNotKnown(appName)}
	}

	if len(app.BoundServices) > 0 {
		for _, bonded := range app.BoundServices {
			bound, err := services.Lookup(cluster, org, bonded)
			if err != nil {
				return APIErrors{InternalError(err)}
			}

			err = app.Unbind(bound)
			if err != nil {
				return APIErrors{InternalError(err)}
			}
		}
	}

	err = app.Delete(gitea)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	// The command above removes the application's deployment.
	// This in turn deletes the associated replicaset, and pod, in
	// this order. The pod being gone thus indicates command
	// completion, and is therefore what we are waiting on below.

	// TODO: Implement a WaitForDeletion on the Application
	err = cluster.WaitForPodBySelectorMissing(nil,
		app.Organization,
		fmt.Sprintf("app.kubernetes.io/name=%s", appName),
		duration.ToDeployment())
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	response := map[string][]string{}
	response["UnboundServices"] = app.BoundServices

	js, err := json.Marshal(response)
	if err != nil {
		return APIErrors{InternalError(err)}
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	return nil
}
