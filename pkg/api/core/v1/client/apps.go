// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport"
	gospdy "k8s.io/client-go/transport/spdy"

	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	progressbar "github.com/schollz/progressbar/v3"
	kubectlterm "k8s.io/kubectl/pkg/util/term"
)

// Upgrader implements the spdy.Upgrader interface. It delegates to spdy.SpdyRoundTripper
// but handles Epinio errors (like 404) first. The upstream upgrader would simply
// ignore 404 in cases when app or namespace is not found.
// Here: https://github.com/kubernetes/apimachinery/blob/v0.21.4/pkg/util/httpstream/spdy/roundtripper.go#L343
type Upgrader struct {
	upstreamUpgr *spdy.SpdyRoundTripper
}

func (upgr *Upgrader) NewConnection(resp *http.Response) (httpstream.Connection, error) {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.New("failed to read response body")
		}

		return nil, errors.New(string(b))
	}

	return upgr.upstreamUpgr.NewConnection(resp)
}

func (upgr *Upgrader) RoundTrip(req *http.Request) (*http.Response, error) {
	return upgr.upstreamUpgr.RoundTrip(req)
}

func NewUpgrader(cfg spdy.RoundTripperConfig) *Upgrader {
	return &Upgrader{upstreamUpgr: spdy.NewRoundTripperWithConfig(cfg)}
}

// AppCreate creates an application resource
func (c *Client) AppCreate(request models.ApplicationCreateRequest, namespace string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("AppCreate", namespace)

	return Post(c, endpoint, request, response)
}

// Apps returns a list of all apps in an namespace
func (c *Client) Apps(namespace string) (models.AppList, error) {
	response := models.AppList{}
	endpoint := api.Routes.Path("Apps", namespace)

	return Get(c, endpoint, response)
}

// AllApps returns a list of all apps
func (c *Client) AllApps() (models.AppList, error) {
	response := models.AppList{}
	endpoint := api.Routes.Path("AllApps")

	return Get(c, endpoint, response)
}

// AppShow shows an app
func (c *Client) AppShow(namespace string, appName string) (models.App, error) {
	response := models.App{}
	endpoint := api.Routes.Path("AppShow", namespace, appName)

	return Get(c, endpoint, response)
}

// AppGetPart retrieves part of an app (values.yaml, chart, image)
func (c *Client) AppGetPart(namespace, appName, part, destinationPath string) error {

	endpoint := api.Routes.Path("AppPart", namespace, appName, part)
	requestBody := ""
	method := "GET"

	// inlined c.get/c.do to the where the response is handled.
	uri := fmt.Sprintf("%s%s/%s", c.Settings.API, api.Root, endpoint)
	c.log.Info(fmt.Sprintf("%s %s", method, uri))

	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		c.log.V(1).Error(err, "cannot build request")
		return err
	}

	err = c.handleAuthorization(request)
	if err != nil {
		return errors.Wrap(err, "handling oauth2 request")
	}

	for key, value := range c.customHeaders {
		request.Header.Set(key, value)
	}

	reqLog := requestLogger(c.log, request, requestBody)

	response, err := c.HttpClient.Do(request)
	if err != nil {
		reqLog.V(1).Error(err, "request failed")
		castedErr, ok := err.(*url.Error)
		if !ok {
			return errors.New("couldn't cast request Error!")
		}
		if castedErr.Timeout() {
			return errors.New("request cancelled or timed out")
		}

		return errors.Wrap(err, "making the request")
	}

	if response.StatusCode != http.StatusOK {
		return handleError(c.log, response)
	}

	defer response.Body.Close()
	reqLog.V(1).Info("request finished")

	// Create the file
	out, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer out.Close()

	contentLength := response.ContentLength
	if response.Header.Get("X-Content-Length") != "" {
		xContentLength, err := strconv.ParseInt(response.Header.Get("X-Content-Length"), 10, 64)
		if err == nil {
			contentLength = xContentLength
		}
	}

	bar := progressbar.DefaultBytes(
		contentLength,
		fmt.Sprintf("Downloading app %s to '%s'", part, destinationPath),
	)

	// copy response body to file
	_, err = io.Copy(io.MultiWriter(out, bar), response.Body)

	c.log.V(1).Info("response stored")

	return err
}

// AppUpdate updates an app
func (c *Client) AppUpdate(request models.ApplicationUpdateRequest, namespace string, appName string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("AppUpdate", namespace, appName)

	return Patch(c, endpoint, request, response)
}

// AppMatch returns all matching namespaces for the prefix
func (c *Client) AppMatch(namespace, prefix string) (models.AppMatchResponse, error) {
	response := models.AppMatchResponse{}
	endpoint := api.Routes.Path("AppMatch", namespace, prefix)

	return Get(c, endpoint, response)
}

// AppDelete deletes an app
func (c *Client) AppDelete(namespace string, names []string) (models.ApplicationDeleteResponse, error) {
	response := models.ApplicationDeleteResponse{}

	queryParams := url.Values{}
	for _, appName := range names {
		queryParams.Add("applications[]", appName)
	}

	endpoint := fmt.Sprintf(
		"%s?%s",
		api.Routes.Path("AppBatchDelete", namespace),
		queryParams.Encode(),
	)

	return Delete(c, endpoint, nil, response)
}

// AppUpload uploads a tarball for the named app, which is later used in staging
func (c *Client) AppUpload(namespace string, name string, tarball string) (models.UploadResponse, error) {
	resp := models.UploadResponse{}

	data, err := c.upload(api.Routes.Path("AppUpload", namespace, name), tarball)
	if err != nil {
		return resp, errors.Wrap(err, "can't upload archive")
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AppValidateCV validates the chart values of the specified app against its appchart
func (c *Client) AppValidateCV(namespace string, name string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("AppValidateCV", namespace, name)

	return Get(c, endpoint, response)
}

// AppImportGit asks the server to import a git repo and put in into the blob store
func (c *Client) AppImportGit(app models.AppRef, gitRef models.GitRef) (*models.ImportGitResponse, error) {
	data := url.Values{}
	data.Set("giturl", gitRef.URL)
	data.Set("gitrev", gitRef.Revision)

	url := fmt.Sprintf("%s%s/%s", c.Settings.API, api.Root, api.Routes.Path("AppImportGit", app.Namespace, app.Name))
	request, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "constructing the request")
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	err = c.handleAuthorization(request)
	if err != nil {
		return nil, errors.Wrap(err, "handling oauth2 request")
	}

	response, err := c.HttpClient.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "making the request to import git")
	}

	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading the response body")
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected server status code: %s\n%s", http.StatusText(response.StatusCode),
			string(bodyBytes))
	}

	resp := &models.ImportGitResponse{}
	if err := json.Unmarshal(bodyBytes, resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AppStage stages an app
func (c *Client) AppStage(request models.StageRequest) (*models.StageResponse, error) {
	response := &models.StageResponse{}
	endpoint := api.Routes.Path("AppStage", request.App.Namespace, request.App.Name)

	return Post(c, endpoint, request, response)
}

// AppDeploy deploys a staged app
func (c *Client) AppDeploy(request models.DeployRequest) (*models.DeployResponse, error) {
	response := &models.DeployResponse{}
	endpoint := api.Routes.Path("AppDeploy", request.App.Namespace, request.App.Name)

	return Post(c, endpoint, request, response)
}

// AppLogs streams the logs of all the application instances, in the targeted namespace
// If stageID is an empty string, runtime application logs are streamed. If stageID
// is set, then the matching staging logs are streamed.
// Logs are streamed through the returned channel.
// There are 2 ways of stopping this method:
// 1. The websocket connection closes.
// 2. The context is canceled (used by the caller when printing of logs should be stopped).
func (c *Client) AppLogs(namespace, appName, stageID string, follow bool, printCallback func(tailer.ContainerLogLine)) error {

	token, err := c.AuthToken()
	if err != nil {
		return err
	}

	queryParams := url.Values{}
	queryParams.Add("follow", strconv.FormatBool(follow))
	queryParams.Add("stage_id", stageID)
	queryParams.Add("authtoken", token)

	var endpoint string
	if stageID == "" {
		endpoint = api.WsRoutes.Path("AppLogs", namespace, appName)
	} else {
		endpoint = api.WsRoutes.Path("StagingLogs", namespace, stageID)
	}

	websocketURL := fmt.Sprintf("%s%s/%s?%s", c.Settings.WSS, api.WsRoot, endpoint, queryParams.Encode())
	webSocketConn, resp, err := websocket.DefaultDialer.Dial(websocketURL, http.Header{})
	if err != nil {
		// Report detailed error found in the server response
		if resp != nil && resp.StatusCode != http.StatusOK {
			return handleError(c.log, resp)
		}

		// Report the dialer error if response claimed to be OK
		return errors.Wrap(err, fmt.Sprintf("Failed to connect to websockets endpoint. Response was = %+v\nThe error is", resp))
	}

	var logLine tailer.ContainerLogLine
	for {
		_, message, err := webSocketConn.ReadMessage()
		if err != nil {
			return nil
		}

		if err := json.Unmarshal(message, &logLine); err != nil {
			return errors.Wrap(err, "error parsing staging message")
		}

		printCallback(logLine)
	}
}

// StagingComplete checks if the staging process is complete
func (c *Client) StagingComplete(namespace string, id string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("StagingComplete", namespace, id)

	return Get(c, endpoint, response)
}

// AppRunning checks if the app is running
func (c *Client) AppRunning(app models.AppRef) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("AppRunning", app.Namespace, app.Name)

	return Get(c, endpoint, response)
}

func (c *Client) AppExec(ctx context.Context, namespace string, appName, instance string, tty kubectlterm.TTY) error {
	endpoint := fmt.Sprintf("%s%s/%s",
		c.Settings.API, api.WsRoot, api.WsRoutes.Path("AppExec", namespace, appName))

	upgradeRoundTripper := NewUpgrader(spdy.RoundTripperConfig{
		TLS:        http.DefaultTransport.(*http.Transport).TLSClientConfig, // See `ExtendLocalTrust`
		PingPeriod: time.Second * 5,
	})

	execURL, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	if err := c.addAuthTokenToURL(execURL); err != nil {
		return err
	}

	if instance != "" {
		values := execURL.Query()
		values.Add("instance", instance)
		execURL.RawQuery = values.Encode()
	}

	// upgradeRoundTripper implements both interfaces, Roundtripper and Upgrader
	exec, err := remotecommand.NewSPDYExecutorForTransports(upgradeRoundTripper, upgradeRoundTripper, "GET", execURL)
	if err != nil {
		return err
	}

	fn := func() error {
		options := remotecommand.StreamOptions{
			Stdin:             tty.In,
			Stdout:            tty.Out,
			Stderr:            tty.Out, // Not used when tty. Check `exec.Stream` docs.
			Tty:               tty.Raw,
			TerminalSizeQueue: tty.MonitorSize(tty.GetSize()),
		}

		return exec.StreamWithContext(ctx, options)
	}

	return tty.Safe(fn)
}

type PortForwardOpts struct {
	Address      []string
	Ports        []string
	StopChannel  chan struct{}
	ReadyChannel chan struct{}
	Out          io.Writer
	ErrOut       io.Writer
}

func NewPortForwardOpts(address, ports []string) *PortForwardOpts {
	opts := &PortForwardOpts{
		Address:      address,
		Ports:        ports,
		StopChannel:  make(chan struct{}),
		ReadyChannel: make(chan struct{}),
		Out:          os.Stdin,
		ErrOut:       os.Stderr,
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if opts.StopChannel != nil {
			close(opts.StopChannel)
		}
	}()

	return opts
}

// AppPortForward will forward the local traffic to a remote app
func (c *Client) AppPortForward(namespace string, appName, instance string, opts *PortForwardOpts) error {
	endpoint := fmt.Sprintf("%s%s/%s", c.Settings.API, api.WsRoot, api.WsRoutes.Path("AppPortForward", namespace, appName))
	portForwardURL, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	if err := c.addAuthTokenToURL(portForwardURL); err != nil {
		return err
	}

	if instance != "" {
		values := portForwardURL.Query()
		values.Add("instance", instance)
		portForwardURL.RawQuery = values.Encode()
	}

	upgradeRoundTripper := NewUpgrader(spdy.RoundTripperConfig{
		TLS:        http.DefaultTransport.(*http.Transport).TLSClientConfig, // See `ExtendLocalTrust`
		PingPeriod: time.Second * 5,
	})

	wrapper := transport.NewBearerAuthRoundTripper(c.Settings.Token.AccessToken, upgradeRoundTripper)

	dialer := gospdy.NewDialer(upgradeRoundTripper, &http.Client{Transport: wrapper}, "GET", portForwardURL)
	fw, err := portforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, opts.Out, opts.ErrOut)
	if err != nil {
		return err
	}

	return fw.ForwardPorts()
}

func (c *Client) addAuthTokenToURL(url *url.URL) error {
	token, err := c.AuthToken()
	if err != nil {
		return err
	}

	values := url.Query()
	values.Add("authtoken", token)
	url.RawQuery = values.Encode()

	return nil
}

// AppRestart restarts an app
func (c *Client) AppRestart(namespace string, appName string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("AppRestart", namespace, appName)

	return Post(c, endpoint, nil, response)
}

func (c *Client) AuthToken() (string, error) {
	response := models.AuthTokenResponse{}
	endpoint := api.Routes.Path("AuthToken")

	tokenResponse, err := Get(c, endpoint, response)
	if err != nil {
		return "", err
	}

	return tokenResponse.Token, nil
}
