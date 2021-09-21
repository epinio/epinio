package v1

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/epinio/epinio/pkg/epinioapi/v1/models"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/julienschmidt/httprouter"
)

// ImportGit handles the API endpoint /namespaces/:org/applications/:app/import-git.
// It receives a Git repo url and revision, clones that (shallow clone), creates a tarball
// of the repo and puts it on S3.
func (hc ApplicationsController) ImportGit(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	name := params.ByName("app")

	url := r.FormValue("giturl")
	revision := r.FormValue("gitrev")

	gitRepo, err := ioutil.TempDir("", "epinio-app")
	if err != nil {
		return InternalError(err, "can't create temp directory")
	}
	defer os.RemoveAll(gitRepo)

	// Fetch the git repo
	// TODO: This is pulling the git repository on user request (synchronously).
	// This can be a slow process. A solution with background workers would be
	// more appropriate. The "pull from git" feature may be redesigned and implemented
	// through an "external" component that monitors git repos. In that case this code
	// will be removed.
	_, err = git.PlainCloneContext(ctx, gitRepo, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.NewBranchReferenceName(revision),
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		return InternalError(err, fmt.Sprintf("cloning the git repository: %s, revision: %s", url, revision))
	}

	// Create a tarball
	tmpDir, tarball, err := helpers.Tar(gitRepo)
	defer func() {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	}()
	if err != nil {
		return InternalError(err, "create a tarball from the git repository")
	}

	// Upload to S3
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err, "failed to get access to a kube client")
	}
	connectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster, deployments.TektonStagingNamespace, deployments.S3ConnectionDetailsSecret)
	if err != nil {
		return InternalError(err, "fetching the S3 connection details from the Kubernetes secret")
	}
	manager, err := s3manager.New(connectionDetails)
	if err != nil {
		return InternalError(err, "creating an S3 manager")
	}

	username, err := GetUsername(r)
	if err != nil {
		return UserNotFound()
	}
	blobUID, err := manager.Upload(ctx, tarball, map[string]string{
		"app": name, "org": org, "username": username,
	})
	if err != nil {
		return InternalError(err, "uploading the application sources blob")
	}
	log.Info("uploaded app", "org", org, "app", name, "blobUID", blobUID)

	// Return response
	resp := models.ImportGitResponse{BlobUID: blobUID}
	err = jsonResponse(w, resp)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
