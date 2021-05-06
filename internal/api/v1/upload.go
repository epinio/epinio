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
)

// Upload receives the application data, as tarball, and creates the gitea as
// well as k8s resources to trigger staging
func (hc ApplicationsController) Upload(w http.ResponseWriter, r *http.Request) []APIError {
	log := tracelog.Logger(r.Context())

	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	app := params.ByName("app")

	log.Info("processing upload for", "org", org, "app", app)

	gitea, err := gitea.New()
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	log.V(2).Info("parsing multipart form")

	file, _, err := r.FormFile("file")
	if err != nil {
		return []APIError{
			NewAPIError("can't read multipart file input: "+err.Error(), "", http.StatusBadRequest),
		}
	}
	defer file.Close()

	tmpDir, err := ioutil.TempDir("", "epinio-app")
	if err != nil {
		return []APIError{
			NewAPIError("can't create temp directory: "+err.Error(), "", http.StatusInternalServerError),
		}
	}
	defer os.RemoveAll(tmpDir)

	blob := path.Join(tmpDir, "blob.tar")
	f, err := os.OpenFile(blob, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return []APIError{
			NewAPIError("failed create file for writing app sources to temp location: "+err.Error(),
				"", http.StatusInternalServerError),
		}
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		return []APIError{
			NewAPIError("failed to copy app sources to temp location"+err.Error(),
				"", http.StatusInternalServerError),
		}
	}

	log.V(2).Info("unpacking temp dir")
	appDir := path.Join(tmpDir, "app")
	err = archiver.Unarchive(blob, appDir)
	if err != nil {
		return []APIError{
			NewAPIError("failed to unpack app sources to temp location"+err.Error(),
				"", http.StatusInternalServerError),
		}
	}

	log.V(2).Info("create gitea app")
	err = gitea.CreateApp(org, app, appDir)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	w.Header().Set("Content-Type", "application/json")
	js, _ := json.Marshal(struct{ Message string }{"ok"})
	_, err = w.Write(js)
	if err != nil {
		return []APIError{NewAPIError(err.Error(), "", http.StatusInternalServerError)}
	}

	return []APIError{}
}
