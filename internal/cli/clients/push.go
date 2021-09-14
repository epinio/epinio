package clients

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/epinio/epinio/helpers"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/duration"
	"github.com/go-logr/logr"
	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
)

func collectSources(log logr.Logger, source string) (string, string, error) {
	files, err := ioutil.ReadDir(source)
	if err != nil {
		return "", "", errors.Wrap(err, "cannot read the apps source files")
	}
	sources := []string{}
	for _, f := range files {
		// The FileInfo entries returned by ReadDir provide
		// only the base name of the file or directory they
		// are for. We have to add back the path of the
		// application directory to get the proper paths to
		// the files and directories to assemble in the
		// tarball.
		// Ignore git config files in the app sources to prevent conflicts with the gitea git repo
		if f.Name() == ".git" || f.Name() == ".gitignore" || f.Name() == ".gitmodules" || f.Name() == ".gitconfig" || f.Name() == ".git-credentials" {
			log.V(3).Info(fmt.Sprintf("Skipping upload of file/dir '%s'.", f.Name()))
			continue
		}
		sources = append(sources, path.Join(source, f.Name()))
	}
	log.V(3).Info("found app data files", "files", sources)

	// create a tmpDir - tarball dir and POST
	tmpDir, err := ioutil.TempDir("", "epinio-app")
	if err != nil {
		return "", "", errors.Wrap(err, "can't create temp directory")
	}

	tarball := path.Join(tmpDir, "blob.tar")
	err = archiver.Archive(sources, tarball)
	if err != nil {
		return tmpDir, "", errors.Wrap(err, "can't create archive")
	}

	return tmpDir, tarball, nil
}

func (c *EpinioClient) uploadCode(app models.AppRef, tarball string) (*models.UploadResponse, error) {
	b, err := c.upload(api.Routes.Path("AppUpload", app.Org, app.Name), tarball)
	if err != nil {
		return nil, errors.Wrap(err, "can't upload archive")
	}

	// returns git commit and app route
	upload := &models.UploadResponse{}
	if err := json.Unmarshal(b, upload); err != nil {
		return nil, err
	}

	return upload, nil
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
