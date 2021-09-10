package clients

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/epinio/epinio/helpers"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/duration"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

func (c *EpinioClient) uploadCode(app models.AppRef, tarball string) (*models.UploadResponse, error) {
	b, err := c.upload(api.Routes.Path("AppUpload", app.Org, app.Name), tarball)
	if err != nil {
		return nil, errors.Wrap(err, "can't upload archive")
	}

	// returns the source blob's UUID
	upload := &models.UploadResponse{}
	if err := json.Unmarshal(b, upload); err != nil {
		return nil, err
	}

	return upload, nil
}

// importGit asks the server to import a git repo and put in into the blob store
func (c *EpinioClient) importGit(app models.AppRef, gitRef models.GitRef) (*models.ImportGitResponse, error) {
	data := url.Values{}
	data.Set("giturl", gitRef.URL)
	data.Set("gitrev", gitRef.Revision)

	url := fmt.Sprintf("%s/%s", c.serverURL, api.Routes.Path("AppImportGit", app.Org, app.Name))
	request, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "constructing the request")
	}
	request.SetBasicAuth(c.Config.User, c.Config.Password)
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

	importResponse := &models.ImportGitResponse{}
	if err := json.Unmarshal(bodyBytes, importResponse); err != nil {
		return nil, err
	}

	return importResponse, nil
}

func (c *EpinioClient) stageCode(req models.StageRequest) (*models.StageResponse, error) {
	out, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshal stage request")
	}

	b, err := c.post(api.Routes.Path("AppStage", req.App.Org, req.App.Name), string(out))
	if err != nil {
		return nil, errors.Wrap(err, "can't stage app")
	}

	// returns staging ID
	stage := &models.StageResponse{}
	if err := json.Unmarshal(b, stage); err != nil {
		return nil, err
	}

	return stage, nil
}

func (c *EpinioClient) stageLogs(details logr.Logger, appRef models.AppRef, stageID string) error {
	// Buffered because the go routine may no longer be listening when we try
	// to stop it. Stopping it should be a fire and forget. We have wg to wait
	// for the routine to be gone.
	stopChan := make(chan bool, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		err := c.AppLogs(appRef.Name, stageID, true, stopChan)
		if err != nil {
			c.ui.Problem().Msg(fmt.Sprintf("failed to tail logs: %s", err.Error()))
		}
	}()

	details.Info("wait for pipelinerun", "StageID", stageID)
	err := c.waitForPipelineRun(appRef, stageID)
	if err != nil {
		stopChan <- true // Stop the printing go routine
		return errors.Wrap(err, "waiting for staging failed")
	}
	stopChan <- true // Stop the printing go routine

	return err
}

func (c *EpinioClient) deployCode(req models.DeployRequest) (*models.DeployResponse, error) {
	out, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshal deploy request")
	}

	b, err := c.post(api.Routes.Path("AppDeploy", req.App.Org, req.App.Name), string(out))
	if err != nil {
		return nil, errors.Wrap(err, "can't deploy app")
	}

	// returns app default route
	deploy := &models.DeployResponse{}
	if err := json.Unmarshal(b, deploy); err != nil {
		return nil, err
	}

	return deploy, nil
}

func (c *EpinioClient) waitForPipelineRun(app models.AppRef, id string) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Running staging")

	return retry.Do(
		func() error {
			out, err := c.get(api.Routes.Path("StagingComplete", app.Org, id))
			return errors.Wrap(err, string(out))
		},
		retry.RetryIf(func(err error) bool {
			return helpers.Retryable(err.Error())
		}),
		retry.OnRetry(func(n uint, err error) {
			c.ui.Note().Msgf("Retrying (%d/%d) after %s", n, duration.RetryMax, err.Error())
		}),
		retry.Delay(time.Second),
		retry.Attempts(duration.RetryMax),
	)
}

func (c *EpinioClient) waitForApp(app models.AppRef) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Creating application resources")

	return retry.Do(
		func() error {
			_, err := c.get(api.Routes.Path("AppRunning", app.Org, app.Name))
			return err
		},
		retry.RetryIf(func(err error) bool {
			return helpers.Retryable(err.Error())
		}),
		retry.OnRetry(func(n uint, err error) {
			c.ui.Note().Msgf("Retrying (%d/%d) after %s", n, duration.RetryMax, err.Error())
		}),
		retry.Delay(time.Second),
		retry.Attempts(duration.RetryMax),
	)
}
