package client

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport"
	gospdy "k8s.io/client-go/transport/spdy"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
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
		b, err := ioutil.ReadAll(resp.Body)
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
func (c *Client) AppCreate(req models.ApplicationCreateRequest, namespace string) (models.Response, error) {
	var resp models.Response

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("AppCreate", namespace), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// Apps returns a list of all apps in an namespace
func (c *Client) Apps(namespace string) (models.AppList, error) {
	var resp models.AppList

	data, err := c.get(api.Routes.Path("Apps", namespace))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AllApps returns a list of all apps
func (c *Client) AllApps() (models.AppList, error) {
	var resp models.AppList

	data, err := c.get(api.Routes.Path("AllApps"))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AppShow shows an app
func (c *Client) AppShow(namespace string, appName string) (models.App, error) {
	var resp models.App

	data, err := c.get(api.Routes.Path("AppShow", namespace, appName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AppGetPart retrieves part of an app (values.yaml, chart, image)
func (c *Client) AppGetPart(namespace, appName, part, destinationPath string) error {

	endpoint := api.Routes.Path("AppPart", namespace, appName, part)
	requestBody := ""
	method := "GET"

	// inlined c.get/c.do to the where the response is handled.
	uri := fmt.Sprintf("%s%s/%s", c.Settings.API, api.Root, endpoint)
	c.log.Info(fmt.Sprintf("%s %s", method, uri))

	reqLog := requestLogger(c.log, method, uri, requestBody)

	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		reqLog.V(1).Error(err, "cannot build request")
		return err
	}

	err = c.handleOauth2Transport()
	if err != nil {
		return errors.Wrap(err, "handling oauth2 request")
	}

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
		bodyBytes, _ := ioutil.ReadAll(response.Body)
		return wrapResponseError(fmt.Errorf("server status code: %s\n%s",
			http.StatusText(response.StatusCode), string(bodyBytes)),
			response.StatusCode)
	}

	defer response.Body.Close()
	reqLog.V(1).Info("request finished")

	// Create the file
	out, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// copy response body to file
	_, err = io.Copy(out, response.Body)

	c.log.V(1).Info("response stored")

	return err
}

// AppUpdate updates an app
func (c *Client) AppUpdate(req models.ApplicationUpdateRequest, namespace string, appName string) (models.Response, error) {
	var resp models.Response

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.patch(api.Routes.Path("AppUpdate", namespace, appName), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AppMatch returns all matching namespaces for the prefix
func (c *Client) AppMatch(namespace, prefix string) (models.AppMatchResponse, error) {
	resp := models.AppMatchResponse{}

	data, err := c.get(api.Routes.Path("AppMatch", namespace, prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AppDelete deletes an app
func (c *Client) AppDelete(namespace string, name string) (models.ApplicationDeleteResponse, error) {
	resp := models.ApplicationDeleteResponse{}

	data, err := c.delete(api.Routes.Path("AppDelete", namespace, name))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
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

	err = c.handleOauth2Transport()
	if err != nil {
		return nil, errors.Wrap(err, "handling oauth2 request")
	}

	response, err := c.HttpClient.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "making the request to import git")
	}

	defer response.Body.Close()
	bodyBytes, err := ioutil.ReadAll(response.Body)
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
func (c *Client) AppStage(req models.StageRequest) (*models.StageResponse, error) {
	out, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshal stage request")
	}

	b, err := c.post(api.Routes.Path("AppStage", req.App.Namespace, req.App.Name), string(out))
	if err != nil {
		return nil, errors.Wrap(err, "can't stage app")
	}

	// returns staging ID
	resp := &models.StageResponse{}
	if err := json.Unmarshal(b, resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AppDeploy deploys a staged app
func (c *Client) AppDeploy(req models.DeployRequest) (*models.DeployResponse, error) {
	out, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshal deploy request")
	}

	b, err := c.post(api.Routes.Path("AppDeploy", req.App.Namespace, req.App.Name), string(out))
	if err != nil {
		return nil, errors.Wrap(err, "can't deploy app")
	}

	// returns app default route
	resp := &models.DeployResponse{}
	if err := json.Unmarshal(b, resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
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
			defer resp.Body.Close()
			bodyBytes, errBody := ioutil.ReadAll(resp.Body)

			if errBody != nil {
				return errBody
			}

			return formatError(bodyBytes, resp)
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
	resp := models.Response{}

	details := c.log.V(1)
	var (
		data []byte
		err  error
	)
	err = retry.Do(
		func() error {
			data, err = c.get(api.Routes.Path("StagingComplete", namespace, id))
			return err
		},
		retry.RetryIf(func(err error) bool {
			// Bail out early when staging failed - Do not retry
			if strings.Contains(err.Error(), "Failed to stage") {
				return false
			}
			if r, ok := err.(interface{ StatusCode() int }); ok {
				return helpers.RetryableCode(r.StatusCode())
			}
			retry := helpers.Retryable(err.Error())

			details.Info("create error", "error", err.Error(), "retry", retry)
			return retry
		}),
		retry.OnRetry(func(n uint, err error) {
			details.WithValues(
				"tries", fmt.Sprintf("%d/%d", n, duration.RetryMax),
				"error", err.Error(),
			).Info("Retrying StagingComplete")
		}),
		retry.Delay(time.Second),
		retry.Attempts(duration.RetryMax),
	)
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AppRunning checks if the app is running
func (c *Client) AppRunning(app models.AppRef) (models.Response, error) {
	resp := models.Response{}

	details := c.log.V(1)
	var (
		data []byte
		err  error
	)
	err = retry.Do(
		func() error {
			data, err = c.get(api.Routes.Path("AppRunning", app.Namespace, app.Name))
			return err
		},
		retry.RetryIf(func(err error) bool {
			if r, ok := err.(interface{ StatusCode() int }); ok {
				return helpers.RetryableCode(r.StatusCode())
			}
			retry := helpers.Retryable(err.Error())

			details.Info("create error", "error", err.Error(), "retry", retry)
			return retry
		}),
		retry.OnRetry(func(n uint, err error) {
			details.WithValues(
				"tries", fmt.Sprintf("%d/%d", n, duration.RetryMax),
				"error", err.Error(),
			).Info("Retrying AppRunning")
		}),
		retry.Delay(time.Second),
		retry.Attempts(duration.RetryMax),
	)
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

func (c *Client) AppExec(namespace string, appName, instance string, tty kubectlterm.TTY) error {
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

		return exec.Stream(options)
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

	// TODO: does it work?
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
func (c *Client) AppRestart(namespace string, appName string) error {
	endpoint := api.Routes.Path("AppRestart", namespace, appName)

	if _, err := c.post(endpoint, ""); err != nil {
		errorMsg := fmt.Sprintf("error restarting app %s in namespace %s", appName, namespace)
		return errors.Wrap(err, errorMsg)
	}

	return nil
}
