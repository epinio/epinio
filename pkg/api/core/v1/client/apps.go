// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
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
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	wsstream "k8s.io/client-go/transport/websocket"

	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	v1 "k8s.io/api/core/v1"
	kubectlterm "k8s.io/kubectl/pkg/util/term"
)

// createRestConfigForWebSocket creates a rest.Config for WebSocket connections
// from the client's API settings. The server will proxy this to Kubernetes API.
func (c *Client) createRestConfigForWebSocket() (*rest.Config, error) {
	baseURL, err := url.Parse(c.Settings.API)
	if err != nil {
		return nil, errors.Wrap(err, "parsing API URL")
	}

	restConfig := &rest.Config{
		Host:    baseURL.Host,
		APIPath: baseURL.Path,
	}

	// Set TLS config from default transport
	if httpTransport, ok := http.DefaultTransport.(*http.Transport); ok && httpTransport.TLSClientConfig != nil {
		restConfig.TLSClientConfig = rest.TLSClientConfig{
			Insecure:   httpTransport.TLSClientConfig.InsecureSkipVerify,
			ServerName: httpTransport.TLSClientConfig.ServerName,
		}
	}

	return restConfig, nil
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
func (c *Client) AppGetPart(namespace, appName, part string) (models.AppPartResponse, error) {
	response := models.AppPartResponse{}
	endpoint := api.Routes.Path("AppPart", namespace, appName, part)

	httpResponse, err := c.Do(endpoint, http.MethodGet, nil)
	if err != nil {
		return response, errors.Wrap(err, "executing AppPart request")
	}

	return models.AppPartResponse{
		Data:          httpResponse.Body,
		ContentLength: httpResponse.ContentLength,
	}, nil
}

// AppExport triggers an export of the app to a registry
func (c *Client) AppExport(namespace, appName string, request models.AppExportRequest) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("AppExport", namespace, appName)

	return Post(c, endpoint, request, response)
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
func (c *Client) AppDelete(namespace string, names []string, deleteImage bool) (models.ApplicationDeleteResponse, error) {
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

	request := models.ApplicationDeleteRequest{
		DeleteImage: deleteImage,
	}

	return Delete(c, endpoint, request, response)
}

// AppUpload uploads a tarball for the named app, which is later used in staging
func (c *Client) AppUpload(namespace string, name string, file FormFile) (models.UploadResponse, error) {
	response := models.UploadResponse{}
	endpoint := api.Routes.Path("AppUpload", namespace, name)

	requestHandler := NewFileUploadRequestHandler(file)
	responseHandler := NewJSONResponseHandler(c.log, response)

	return DoWithHandlers(c, endpoint, http.MethodPost, requestHandler, responseHandler)
}

// AppValidateCV validates the chart values of the specified app against its appchart
func (c *Client) AppValidateCV(namespace string, name string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("AppValidateCV", namespace, name)

	return Get(c, endpoint, response)
}

// AppImportGit asks the server to import a git repo and put in into the blob store
func (c *Client) AppImportGit(namespace string, name string, gitRef models.GitRef) (models.ImportGitResponse, error) {
	response := models.ImportGitResponse{}
	endpoint := api.Routes.Path("AppImportGit", namespace, name)

	data := url.Values{}
	data.Set("giturl", gitRef.URL)
	data.Set("gitrev", gitRef.Revision)

	requestHandler := NewFormURLEncodedRequestHandler(data)
	responseHandler := NewJSONResponseHandler(c.log, response)

	return DoWithHandlers(c, endpoint, http.MethodPost, requestHandler, responseHandler)
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

// LogOptions represents the optional filters for retrieving application logs.
type LogOptions struct {
	Tail              *int64
	Since             *time.Duration
	SinceTime         *time.Time
	IncludeContainers []string // List of container names/patterns to include (regex patterns supported)
	ExcludeContainers []string // List of container names/patterns to exclude (regex patterns supported)
}

// AppLogs streams the logs of all the application instances, in the targeted namespace
// If stageID is an empty string, runtime application logs are streamed. If stageID
// is set, then the matching staging logs are streamed.
// Logs are streamed through the returned channel.
// There are 2 ways of stopping this method:
// 1. The websocket connection closes.
// 2. The context is canceled (used by the caller when printing of logs should be stopped).
func (c *Client) AppLogs(namespace, appName, stageID string, follow bool, options *LogOptions, printCallback func(tailer.ContainerLogLine)) error {

	tokenResponse, err := c.AuthToken()
	if err != nil {
		return err
	}

	queryParams := url.Values{}
	queryParams.Add("follow", strconv.FormatBool(follow))
	if stageID != "" {
		queryParams.Add("stage_id", stageID)
	}
	queryParams.Add("authtoken", tokenResponse.Token)

	if options != nil {
		if options.Tail != nil {
			queryParams.Add("tail", strconv.FormatInt(*options.Tail, 10))
		}
		if options.Since != nil {
			queryParams.Add("since", options.Since.String())
		}
		if options.SinceTime != nil {
			queryParams.Add("since_time", options.SinceTime.Format(time.RFC3339))
		}
		if len(options.IncludeContainers) > 0 {
			queryParams.Add("include_containers", strings.Join(options.IncludeContainers, ","))
		}
		if len(options.ExcludeContainers) > 0 {
			queryParams.Add("exclude_containers", strings.Join(options.ExcludeContainers, ","))
		}
	}

	var endpoint string
	if stageID == "" {
		endpoint = api.WsRoutes.Path("AppLogs", namespace, appName)
	} else {
		endpoint = api.WsRoutes.Path("StagingLogs", namespace, stageID)
	}

	websocketURL := fmt.Sprintf("%s%s/%s?%s", c.Settings.WSS, api.WsRoot, endpoint, queryParams.Encode())
	webSocketConn, resp, err := websocket.DefaultDialer.Dial(websocketURL, c.Headers())
	if err != nil {
		// Report detailed error found in the server response
		if resp != nil && resp.StatusCode != http.StatusOK {
			return handleError(c.log, resp)
		}

		// Report the dialer error if response claimed to be OK
		return errors.Wrap(err, fmt.Sprintf("Failed to connect to websockets endpoint. Response was = %+v\nThe error is", resp))
	}
	defer func() {
		if err := webSocketConn.Close(); err != nil {
			c.log.V(1).Error(err, "failed to close websocket connection")
		}
	}()

	var logLine tailer.ContainerLogLine
	for {
		_, message, err := webSocketConn.ReadMessage()
		if err != nil {
			// Connection closed or error - this is normal when logs stream ends
			// Return nil to indicate normal completion
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			// For other errors, log but still return nil (stream ended)
			c.log.V(1).Error(err, "websocket read error, ending log stream")
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

// StagingCompleteStream opens a websocket that emits a single completion event
// for the given staging run and closes once the job finishes.
func (c *Client) StagingCompleteStream(ctx context.Context, namespace, id string, callback func(models.StageCompleteEvent) error) error {
	tokenResponse, err := c.AuthToken()
	if err != nil {
		return err
	}

	endpoint := api.WsRoutes.Path("StagingCompleteWs", namespace, id)
	queryParams := url.Values{}
	queryParams.Add("authtoken", tokenResponse.Token)
	websocketURL := fmt.Sprintf("%s%s/%s?%s", c.Settings.WSS, api.WsRoot, endpoint, queryParams.Encode())

	webSocketConn, resp, err := websocket.DefaultDialer.DialContext(ctx, websocketURL, c.Headers())
	if err != nil {
		if resp != nil && resp.StatusCode != http.StatusOK {
			return handleError(c.log, resp)
		}
		return errors.Wrap(err, "failed to connect to staging completion websocket")
	}
	defer func() { _ = webSocketConn.Close() }()

	for {
		_, message, readErr := webSocketConn.ReadMessage()
		if readErr != nil {
			// Normal close means the server is done sending updates.
			if websocket.IsCloseError(readErr, websocket.CloseNormalClosure) {
				return nil
			}
			return errors.Wrap(readErr, "reading staging completion websocket message")
		}

		var event models.StageCompleteEvent
		if unmarshalErr := json.Unmarshal(message, &event); unmarshalErr != nil {
			return errors.Wrap(unmarshalErr, "decoding staging completion event")
		}

		if callback != nil {
			if cbErr := callback(event); cbErr != nil {
				return cbErr
			}
		}

		if event.Completed {
			return nil
		}
	}
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

	// Create rest.Config for WebSocket executor
	restConfig, err := c.createRestConfigForWebSocket()
	if err != nil {
		return err
	}

	// Use WebSocket executor instead of SPDY
	// The full URL with auth token is passed as the url parameter
	exec, err := remotecommand.NewWebSocketExecutor(restConfig, "GET", execURL.String())
	if err != nil {
		return errors.Wrap(err, "creating websocket executor")
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

// WebSocketDialer implements httpstream.Dialer for WebSocket connections
type WebSocketDialer struct {
	restConfig *rest.Config
	method     string
	url        *url.URL
}

// NewWebSocketDialer creates a new WebSocket dialer for port forwarding
func NewWebSocketDialer(restConfig *rest.Config, method string, url *url.URL) httpstream.Dialer {
	return &WebSocketDialer{
		restConfig: restConfig,
		method:     method,
		url:        url,
	}
}

// Dial implements httpstream.Dialer interface
func (d *WebSocketDialer) Dial(protocols ...string) (httpstream.Connection, string, error) {
	// Validate protocols
	if len(protocols) == 0 {
		return nil, "", errors.New("at least one protocol must be specified")
	}

	// Create HTTP request
	req, err := http.NewRequest(d.method, d.url.String(), nil)
	if err != nil {
		return nil, "", errors.Wrap(err, "creating request")
	}

	// Get WebSocket roundtripper from rest config
	rt, connHolder, err := wsstream.RoundTripperFor(d.restConfig)
	if err != nil {
		return nil, "", errors.Wrap(err, "creating websocket roundtripper")
	}

	// Execute request to establish WebSocket connection
	resp, err := rt.RoundTrip(req)
	if err != nil {
		return nil, "", errors.Wrap(err, "round trip failed")
	}
	// Ensure response body is closed in all paths
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	// Get WebSocket connection from connection holder
	wsConn, err := wsstream.Negotiate(rt, connHolder, req, protocols...)
	if err != nil {
		return nil, "", errors.Wrap(err, "websocket negotiation failed")
	}

	// Convert WebSocket connection to httpstream.Connection
	streamConn := &websocketStreamConnection{
		conn:       wsConn,
		connHolder: connHolder,
		streams:    make(map[uint32]*websocketStream),
		closeCh:    make(chan bool, 1),
		readerDone: make(chan struct{}),
	}

	// Start a single reader that routes data to streams
	go streamConn.readerLoop()

	return streamConn, protocols[0], nil
}

// websocketStreamConnection wraps a WebSocket connection to implement httpstream.Connection
type websocketStreamConnection struct {
	conn       *websocket.Conn
	connHolder wsstream.ConnectionHolder
	streams    map[uint32]*websocketStream
	nextID     uint32
	mu         sync.Mutex
	closeCh    chan bool
	readerDone chan struct{}
}

func (w *websocketStreamConnection) CreateStream(headers http.Header) (httpstream.Stream, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.streams == nil {
		w.streams = make(map[uint32]*websocketStream)
	}

	// Generate stream ID
	w.nextID++
	streamID := w.nextID

	// Create a new stream with larger buffer for high-throughput port forwarding
	stream := &websocketStream{
		id:      streamID,
		headers: headers,
		conn:    w.conn,
		readCh:  make(chan []byte, 1000), // Increased buffer size for high throughput
		done:    make(chan struct{}),
	}

	w.streams[streamID] = stream

	return stream, nil
}

// readerLoop reads from the WebSocket connection and routes data to streams.
// Kubernetes port forwarding protocol: Data flows directly through WebSocket.
// The portforward package handles protocol parsing (port headers, request IDs).
// We route all data to data streams, and errors to error streams.
func (w *websocketStreamConnection) readerLoop() {
	defer close(w.readerDone)
	for {
		messageType, data, err := w.conn.ReadMessage()
		if err != nil {
			// Connection closed or error - signal all streams
			w.mu.Lock()
			for _, stream := range w.streams {
				// Use select to avoid panic if channel already closed
				select {
				case <-stream.done:
					// Already closed, skip
				default:
					close(stream.readCh)
				}
			}
			w.mu.Unlock()
			return
		}

		if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
			w.mu.Lock()
			// For Kubernetes port forwarding, data flows through the connection.
			// The portforward package will parse the protocol (port headers, request IDs).
			// We route based on stream type headers set when streams were created.
			// Note: When multiple data streams exist (multiple ports), the same data
			// is sent to all data streams. The portforward package filters by port
			// headers in the data, so this is the correct behavior.
			var dataStreams []*websocketStream
			var errorStreams []*websocketStream

			for _, stream := range w.streams {
				switch streamType := stream.headers.Get(v1.StreamType); streamType {
				case v1.StreamTypeData:
					dataStreams = append(dataStreams, stream)
				case v1.StreamTypeError:
					errorStreams = append(errorStreams, stream)
				}
			}

			// Route data: binary messages to data streams, text/errors to error streams
			// If no streams exist yet, data is dropped (streams should be created before data arrives)
			if messageType == websocket.BinaryMessage {
				// Binary data goes to data streams (port forwarding data)
				for _, stream := range dataStreams {
					select {
					case stream.readCh <- data:
					case <-stream.done:
						// Stream closed, skip
					default:
						// Channel full, skip to avoid blocking
					}
				}
				// If no data streams, also try error streams (fallback)
				if len(dataStreams) == 0 && len(errorStreams) > 0 {
					for _, stream := range errorStreams {
						select {
						case stream.readCh <- data:
						case <-stream.done:
						default:
						}
					}
				}
			} else {
				// Text messages typically indicate errors
				for _, stream := range errorStreams {
					select {
					case stream.readCh <- data:
					case <-stream.done:
					default:
					}
				}
				// Fallback to data streams if no error streams
				if len(errorStreams) == 0 {
					for _, stream := range dataStreams {
						select {
						case stream.readCh <- data:
						case <-stream.done:
						default:
						}
					}
				}
			}
			w.mu.Unlock()
		}
	}
}

// websocketStream implements httpstream.Stream for WebSocket connections
type websocketStream struct {
	id      uint32
	headers http.Header
	conn    *websocket.Conn
	readCh  chan []byte
	done    chan struct{}
	closed  bool
	mu      sync.Mutex
	buffer  []byte // Buffer for partial reads
}

func (s *websocketStream) Read(p []byte) (n int, err error) {
	// First, consume any buffered data
	if len(s.buffer) > 0 {
		n = copy(p, s.buffer)
		s.buffer = s.buffer[n:]
		if len(p) == n {
			return n, nil
		}
		// Still have space, continue to read from channel
	}

	// Read from channel
	select {
	case data := <-s.readCh:
		if len(data) == 0 {
			return 0, io.EOF
		}
		remaining := len(p) - n
		if remaining > 0 {
			copied := copy(p[n:], data)
			n += copied
			if copied < len(data) {
				// Buffer remaining data
				s.buffer = data[copied:]
			}
		}
		return n, nil
	case <-s.done:
		if n > 0 {
			return n, nil
		}
		return 0, io.EOF
	}
}

func (s *websocketStream) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, io.ErrClosedPipe
	}

	// For port forwarding, write directly to WebSocket connection
	// The WebSocket protocol handles the framing
	err = s.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *websocketStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	return nil
}

func (s *websocketStream) Reset() error {
	return s.Close()
}

func (s *websocketStream) Headers() http.Header {
	return s.headers
}

func (s *websocketStream) Identifier() uint32 {
	return s.id
}

func (w *websocketStreamConnection) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Close all streams
	for _, stream := range w.streams {
		_ = stream.Close()
	}
	w.streams = nil

	// Close WebSocket connection
	if w.conn != nil {
		err := w.conn.Close()
		select {
		case w.closeCh <- true:
		default:
		}
		return err
	}
	return nil
}

func (w *websocketStreamConnection) CloseChan() <-chan bool {
	if w.closeCh == nil {
		w.closeCh = make(chan bool, 1)
	}
	return w.closeCh
}

func (w *websocketStreamConnection) RemoteAddr() net.Addr {
	if w.conn != nil {
		return w.conn.RemoteAddr()
	}
	return nil
}

// SetIdleTimeout sets the idle timeout for the connection.
// For WebSocket connections, we set read/write deadlines on the underlying connection.
// Errors from SetReadDeadline/SetWriteDeadline are non-fatal and typically indicate
// the connection is already closed, so we ignore them.
func (w *websocketStreamConnection) SetIdleTimeout(timeout time.Duration) {
	if w.conn != nil && timeout > 0 {
		deadline := time.Now().Add(timeout)
		_ = w.conn.SetReadDeadline(deadline)
		_ = w.conn.SetWriteDeadline(deadline)
	}
}

func (w *websocketStreamConnection) RemoveStreams(streams ...httpstream.Stream) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, stream := range streams {
		if wsStream, ok := stream.(*websocketStream); ok {
			delete(w.streams, wsStream.id)
			_ = wsStream.Close()
		}
	}
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

	// Create rest.Config for WebSocket dialer
	restConfig, err := c.createRestConfigForWebSocket()
	if err != nil {
		return err
	}

	// Create WebSocket dialer instead of SPDY dialer
	dialer := NewWebSocketDialer(restConfig, "GET", portForwardURL)
	fw, err := portforward.NewOnAddresses(dialer, opts.Address, opts.Ports, opts.StopChannel, opts.ReadyChannel, opts.Out, opts.ErrOut)
	if err != nil {
		return err
	}

	return fw.ForwardPorts()
}

func (c *Client) addAuthTokenToURL(url *url.URL) error {
	tokenResponse, err := c.AuthToken()
	if err != nil {
		return err
	}

	values := url.Query()
	values.Add("authtoken", tokenResponse.Token)
	url.RawQuery = values.Encode()

	return nil
}

// AppRestart restarts an app
func (c *Client) AppRestart(namespace string, appName string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("AppRestart", namespace, appName)

	return Post(c, endpoint, nil, response)
}

func (c *Client) AuthToken() (models.AuthTokenResponse, error) {
	response := models.AuthTokenResponse{}
	endpoint := api.Routes.Path("AuthToken")

	return Get(c, endpoint, response)
}
