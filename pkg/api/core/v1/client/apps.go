package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport"

	"github.com/epinio/epinio/helpers"
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

	url := fmt.Sprintf("%s%s/%s", c.URL, api.Root, api.Routes.Path("AppImportGit", app.Namespace, app.Name))
	request, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "constructing the request")
	}
	request.SetBasicAuth(c.user, c.password)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	response, err := (&http.Client{}).Do(request)
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

func (c *Client) AppExec(namespace string, appName string, tty kubectlterm.TTY) error {
	endpoint := fmt.Sprintf("%s%s/%s",
		c.URL, api.Root, api.Routes.Path("AppExec", namespace, appName))

	upgradeRoundTripper := NewUpgrader(spdy.RoundTripperConfig{
		TLS:                      http.DefaultTransport.(*http.Transport).TLSClientConfig, // See `ExtendLocalTrust`
		FollowRedirects:          true,
		RequireSameHostRedirects: false,
		PingPeriod:               time.Second * 5,
	})

	wrapper := transport.NewBasicAuthRoundTripper(c.user, c.password, upgradeRoundTripper)

	execURL, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	exec, err := remotecommand.NewSPDYExecutorForTransports(wrapper, upgradeRoundTripper, "GET", execURL)
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
