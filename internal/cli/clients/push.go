package clients

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"

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
		return nil, errors.Wrap(err, "can't marshal upload response")
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

func (c *EpinioClient) waitForPipelineRun(app models.AppRef, stageId string) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Running staging")

	b, err := c.post(api.Routes.Path("AppUntilStaged", app.Org, app.Name, stageId), "")
	if err != nil {
		return errors.Wrap(err, "failed to wait for app staging to complete")
	}

	// Return the reported staging status
	status := &models.StageStatusResponse{}
	if err := json.Unmarshal(b, status); err != nil {
		return err
	}

	if status.ErrorMessage == "" {
		return nil
	}

	return errors.New(status.ErrorMessage)
}

func (c *EpinioClient) waitForApp(app models.AppRef) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Creating application resources")

	err := c.KubeClient.WaitForDeploymentCompleted(
		c.ui, app.Org, app.Name, duration.ToAppBuilt())
	if err != nil {
		return errors.Wrap(err, "waiting for app to come online failed")
	}

	return nil
}
