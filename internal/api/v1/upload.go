package v1

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/julienschmidt/httprouter"
	"github.com/mholt/archiver/v3"
)

// Upload receives the application data, as tarball, and creates the gitea as
// well as k8s resources to trigger staging
func (hc ApplicationsController) Upload(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	name := params.ByName("app")

	log.Info("processing upload", "org", org, "app", name)

	client, err := gitea.New(ctx)
	if err != nil {
		return InternalError(err)
	}

	log.V(2).Info("parsing multipart form")

	file, _, err := r.FormFile("file")
	if err != nil {
		return BadRequest(err, "can't read multipart file input")
	}
	defer file.Close()

	tmpDir, err := ioutil.TempDir("", "epinio-app")
	if err != nil {
		return InternalError(err, "can't create temp directory")
	}
	defer os.RemoveAll(tmpDir)

	blob := path.Join(tmpDir, "blob.tar")
	f, err := os.OpenFile(blob, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return InternalError(err, "failed create file for writing app sources to temp location")
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		return InternalError(err, "failed to copy app sources to temp location")
	}

	log.V(2).Info("unpacking temp dir")
	appDir := path.Join(tmpDir, "app")
	err = archiver.Unarchive(blob, appDir)
	if err != nil {
		return InternalError(err, "failed to unpack app sources to temp location")
	}

	log.V(2).Info("create gitea app repo")
	app := models.NewAppRef(name, org)
	g, err := client.Upload(app, appDir)
	if err != nil {
		return InternalError(err)
	}

	log.Info("uploaded app", "org", org, "app", name)

	// Extend url to contain the full repo path
	g.URL = fmt.Sprintf("%s/%s/%s", g.URL, org, name)

	resp := models.UploadResponse{Git: &g}
	err = jsonResponse(w, resp)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
