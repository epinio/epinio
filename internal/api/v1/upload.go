package v1

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/julienschmidt/httprouter"
	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
)

// Upload receives the application data, as tarball, and creates the gitea as
// well as k8s resources to trigger staging
func (hc ApplicationsController) Upload(w http.ResponseWriter, r *http.Request) {
	log := tracelog.Logger(r.Context())

	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	app := params.ByName("app")

	log.Info("processing upload for", "org", org, "app", app)

	gitea, err := gitea.New()
	if err != nil {
		handleError(w, err, http.StatusInternalServerError)
		return
	}

	log.V(2).Info("parsing multipart form")

	file, _, err := r.FormFile("file")
	if err != nil {
		err = errors.Wrapf(err, "can't read multipart file input")
		handleError(w, err, http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmpDir, err := ioutil.TempDir("", "epinio-app")
	if err != nil {
		err = errors.Wrapf(err, "can't create temp directory")
		handleError(w, err, http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	blob := path.Join(tmpDir, "blob.tar")
	f, err := os.OpenFile(blob, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		handleError(w, errors.Wrap(err, "failed create file for writing app sources to temp location"), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		handleError(w, errors.Wrap(err, "failed to copy app sources to temp location"), http.StatusInternalServerError)
		return
	}

	log.V(2).Info("unpacking temp dir")
	appDir := path.Join(tmpDir, "app")
	err = archiver.Unarchive(blob, appDir)
	if err != nil {
		handleError(w, errors.Wrap(err, "failed to unpack app sources to temp location"), http.StatusInternalServerError)
		return
	}

	log.V(2).Info("create gitea app")
	err = gitea.CreateApp(org, app, appDir)
	if err != nil {
		handleError(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	js, _ := json.Marshal(struct{ Message string }{"ok"})
	_, err = w.Write(js)
	if err != nil {
		handleError(w, err, http.StatusInternalServerError)
		return
	}
}
