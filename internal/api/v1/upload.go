package v1

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/julienschmidt/httprouter"
	"github.com/mholt/archiver/v3"
)

// Upload handles the API endpoint /orgs/:org/applications/:app/store.
// It receives the application data as a tarball and stores it. Then
// it creates the k8s resources needed for staging
func (hc ApplicationsController) Upload(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	name := params.ByName("app")

	log.Info("processing upload", "org", org, "app", name)

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

	// TODO: Put the code on the PVC here

	log.Info("uploaded app", "org", org, "app", name)

	// TODO: Put the "id" of the uploaded code version (UUID?) in the response
	// to be used for the staging request.
	resp := models.UploadResponse{Git: nil}
	err = jsonResponse(w, resp)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
