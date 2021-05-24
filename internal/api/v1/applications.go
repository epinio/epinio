package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

type ApplicationsController struct {
	conn *websocket.Conn
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

func (hc ApplicationsController) Update(w http.ResponseWriter, r *http.Request) APIErrors {
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

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	var updateRequest models.UpdateAppRequest
	err = json.Unmarshal(bodyBytes, &updateRequest)
	if err != nil {
		return APIErrors{BadRequest(err)}
	}

	if updateRequest.Instances < 0 {
		return APIErrors{NewAPIError(
			"instances param should be integer equal or greater than zero",
			"", http.StatusBadRequest)}
	}

	err = app.Scale(r.Context(), updateRequest.Instances)
	if err != nil {
		return singleError(err, http.StatusInternalServerError)
	}

	return nil
}
func (hc ApplicationsController) Logs(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	queryValues := r.URL.Query()
	followStr := queryValues.Get("follow")
	fmt.Printf("followStr = %+v\n", followStr)

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		jsonErrorResponse(w, InternalError(err))
		return
	}

	var upgrader = websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		jsonErrorResponse(w, InternalError(err))
		return
	}

	// Case - no following
	// Get App Ref
	// app.Logs gives you channel
	// loop over the channel
	// stream get closed
	//

	// Case - follow
	// How to close the channel ?
	// When the user of the api issues close connection.
	// then we close the context

	follow := false
	if followStr == "true" {
		follow = true
	}

	log := tracelog.Logger(r.Context())

	hc.conn = conn
	err = hc.streamPodLogs(org, appName, cluster, follow, r.Context())
	if err != nil {
		log.V(1).Error(err, "error occured after upgrading the websockets connection")
		return
	}
}

func (hc ApplicationsController) streamPodLogs(orgName string, appName string, cluster *kubernetes.Cluster, follow bool, ctx context.Context) error {
	selector := labels.NewSelector()
	for _, req := range [][]string{
		{"app.kubernetes.io/component", "application"},
		{"app.kubernetes.io/managed-by", "epinio"},
		{"app.kubernetes.io/part-of", orgName},
		{"app.kubernetes.io/name", appName},
	} {
		req, err := labels.NewRequirement(req[0], selection.Equals, []string{req[1]})
		if err != nil {
			return err
		}
		selector = selector.Add(*req)
	}

	logCtx, logCancelFunc := context.WithCancel(ctx)
	defer func() {
		logCancelFunc()
	}()
	logChan := make(chan tailer.ContainerLogLine)
	config := &tailer.Config{
		ContainerQuery:        regexp.MustCompile(".*"),
		ExcludeContainerQuery: nil,
		ContainerState:        "running",
		Exclude:               nil,
		Include:               nil,
		Timestamps:            false,
		Since:                 duration.LogHistory(),
		AllNamespaces:         true,
		LabelSelector:         selector,
		TailLines:             nil,
		Namespace:             "",
		PodQuery:              regexp.MustCompile(".*"),
	}

	if follow {
		go func() {
			tailer.StreamLogs(logCtx, logChan, config, cluster)
			close(logChan)
		}()
	} else {
		go func() {
			tailer.FetchLogs(logCtx, logChan, config, cluster)
			close(logChan)
		}()
	}

	// Read logs until channel no logs are left or connection is closed from the
	// client side (which will send an error to the for loop below and it will
	// call the logCancelFunc to stop this routine too)
	for logLine := range logChan {
		msg, err := json.Marshal(logLine)
		if err != nil {
			return err
		}

		err = hc.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			fmt.Println(err.Error())

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				hc.conn.Close()
				return nil
			}
			if websocket.IsUnexpectedCloseError(err) {
				hc.conn.Close()
				fmt.Println(errors.Wrap(err, "error from client"))
				return nil
			}

			normalCloseErr := hc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
			if normalCloseErr != nil {
				err = errors.Wrap(err, normalCloseErr.Error())
			}

			abnormalCloseErr := hc.conn.Close()
			if abnormalCloseErr != nil {
				err = errors.Wrap(err, abnormalCloseErr.Error())
			}

			return err
		}
	}

	if err := hc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{}); err != nil {
		return err
	}

	return hc.conn.Close()
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

	err = application.Delete(cluster, gitea, org, *app)
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
