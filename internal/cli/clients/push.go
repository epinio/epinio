package clients

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path"
	"time"

	"github.com/epinio/epinio/deployments"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/duration"
	"github.com/go-logr/logr"
	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
	tekton "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
			log.V(3).Info("Skipping upload of file/dir '" + f.Name() + "'.")
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

func (c *EpinioClient) waitForPipelineRun(app models.AppRef, id string) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Running staging")

	cs, err := tekton.NewForConfig(c.KubeClient.RestConfig)
	if err != nil {
		return err
	}
	client := cs.TektonV1beta1().PipelineRuns(deployments.TektonStagingNamespace)

	return wait.PollImmediate(time.Second, duration.ToAppBuilt(),
		func() (bool, error) {
			l, err := client.List(context.TODO(), metav1.ListOptions{LabelSelector: models.EpinioStageIDLabel + "=" + id})
			if err != nil {
				return false, err
			}
			if len(l.Items) == 0 {
				return false, nil
			}
			for _, pr := range l.Items {
				// any failed conditions, throw an error so we can exit early
				for _, c := range pr.Status.Conditions {
					if c.IsFalse() {
						return false, errors.New(c.Message)
					}
				}
				// it worked
				if pr.Status.CompletionTime != nil {
					return true, nil
				}
			}
			// pr exists, but still running
			return false, nil
		})
}

func (c *EpinioClient) waitForApp(app models.AppRef, id string) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Creating application resources")

	err := c.KubeClient.WaitForDeploymentCompleted(
		c.ui, app.Org, app.Name, duration.ToAppBuilt())
	if err != nil {
		return errors.Wrap(err, "waiting for app to come online failed")
	}

	return nil
}
