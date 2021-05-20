package v1

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
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
func (hc ApplicationsController) Logs(w http.ResponseWriter, r *http.Request) APIErrors {
	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	appName := params.ByName("app")

	queryValues := r.URL.Query()
	follow := queryValues.Get("follow")

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	var upgrader = websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	hc.conn = conn
	err = hc.streamPodLogs(org, appName, cluster, follow)
	if err != nil {
		return APIErrors{InternalError(err)}
	}

	return nil
}

func (hc ApplicationsController) streamPodLogs(orgName string, appName string, cluster *kubernetes.Cluster, follow string) error {
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

	podList, err := cluster.Kubectl.CoreV1().Pods(orgName).List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	podLogOpts := corev1.PodLogOptions{
		Container: appName,
	}
	if follow == "true" {
		podLogOpts.Follow = true
	}

	errorChan := make(chan error, 1)
	doneChan := make(chan bool)
	quitChan := make(chan bool)

	podLen := len(podList.Items)
	var mu sync.Mutex
	for _, pod := range podList.Items {

		// Call goroutines for each pod replica
		go func(pod corev1.Pod) {
			req := cluster.Kubectl.CoreV1().Pods(orgName).GetLogs(pod.Name, &podLogOpts)
			stream, err := req.Stream(context.Background())
			if err != nil {
				errorChan <- err
				return
			}
			defer stream.Close()

			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				message := fmt.Sprintf("[%s] %s", pod.Name, scanner.Text())

				// websocket connection doesn't support multiple writes
				mu.Lock()
				err = hc.conn.WriteMessage(websocket.TextMessage, []byte(message))
				mu.Unlock()
				if err != nil {
					errorChan <- err
					return
				}
			}

			// Exit goroutine by sending true to done channel
			// Exit goroutine by waiting quit channel
			select {
			case doneChan <- true:
				return
			case <-quitChan:
				return
			}
		}(pod)
	}

	go func() {

		// Websocket closure from peer can be only captured
		// from reading the connection until err occurs
		for {
			if _, _, err := hc.conn.NextReader(); err != nil {
				errorChan <- err
				break
			}
		}

		// Exit goroutine by waiting on quit channel
		select {
		case <-quitChan:
			return
		}
	}()

	// Capture errors and done signals from goroutines spawned above
	countPod := 0
	for {
		select {
		case err := <-errorChan:
			close(quitChan)

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				hc.conn.Close()
				return nil
			}
			if websocket.IsUnexpectedCloseError(err) {
				hc.conn.Close()
				fmt.Println(errors.Wrap(err, "error from client"))
				return nil
			}

			err = hc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
			if err != nil {
				err = hc.conn.Close()
				if err != nil {
					return err
				}
			}
			return err
		case <-doneChan:
			countPod++

			if countPod == podLen {
				close(quitChan)
				err = hc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
				if err != nil {
					err = hc.conn.Close()
					if err != nil {
						return err
					}
				}

				return nil
			}
		}
	}
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
