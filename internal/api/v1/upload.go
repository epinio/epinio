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

type AppResponse struct {
	App     gitea.App `json:"app"`
	Message string    `json:"message"`
}

func NewAppResponse(msg string, app gitea.App) *AppResponse {
	return &AppResponse{Message: msg, App: app}
}

func (r *AppResponse) Write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(AppResponse{Message: "ok", App: r.App})
	if err != nil {
		return err
	}
	_, err = w.Write(js)
	return err
}

// Upload receives the application data, as tarball, and creates the gitea as
// well as k8s resources to trigger staging
func (hc ApplicationsController) Upload(w http.ResponseWriter, r *http.Request) APIErrors {
	log := tracelog.Logger(r.Context())

	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	name := params.ByName("app")

	log.Info("processing upload", "org", org, "app", name)

	client, err := gitea.New()
	if err != nil {
		return NewAPIErrors(InternalError(err))
	}

	log.V(2).Info("parsing multipart form")

	file, _, err := r.FormFile("file")
	if err != nil {
		return NewAPIErrors(BadRequest(err, "can't read multipart file input"))
	}
	defer file.Close()

	tmpDir, err := ioutil.TempDir("", "epinio-app")
	if err != nil {
		return singleInternalError(err, "can't create temp directory")
	}
	defer os.RemoveAll(tmpDir)

	blob := path.Join(tmpDir, "blob.tar")
	f, err := os.OpenFile(blob, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return singleInternalError(err, "failed create file for writing app sources to temp location")
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		return singleInternalError(err, "failed to copy app sources to temp location")
	}

	log.V(2).Info("unpacking temp dir")
	appDir := path.Join(tmpDir, "app")
	err = archiver.Unarchive(blob, appDir)
	if err != nil {
		return singleInternalError(err, "failed to unpack app sources to temp location")
	}

	log.V(2).Info("create gitea app repo")
	app := gitea.App{Name: name, Org: org}
	err = client.Upload(&app, appDir)
	if err != nil {
		return NewAPIErrors(InternalError(err))
	}

	log.Info("uploaded app", "org", org, "app", app)

	err = NewAppResponse("ok", app).Write(w)
	if err != nil {
		return NewAPIErrors(InternalError(err))
	}

	return nil
}
