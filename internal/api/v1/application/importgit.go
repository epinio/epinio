// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

// # Design Notes
//
// ## Possible inputs:
//
//   - (A) repository, no revision (empty string)
//   - (B) repository, branch name
//   - (C) repository, commit id
//
// ## Considerations:
//
//   - Correctness
//   - Cloning performance
//
// The go-git cloning function has two attributes influencing performance
//
//   - `Depth` specifies the depth towards which to clone.
//   - `SingleBranch` specifies the sole branch to check out.
//
// The second flag comes with a problem. Using it __demands__ a branch.  And whatever is
// found in the `ReferenceName` of the CloneOptions is used.  Even if it is an empty
// string. And leaving it completely unspecified makes the package use a hardwired default
// (`master`).
//
// If we have a revision which is a branch name, then we can (try to) use `SingleBranch`.
// Note however, that we cannot syntactically distinguish branch names from commit ids.
//
// ## Solutions
//
// The simplest code handling everything would be
//
//      Clone (Depth=1)
//      if revision:
//          hash = ResolveRevision (revision)
//          Checkout (hash)
//
// with no `SingleBranch` in sight, just `Depth`.
//
// More complex, hopefully more performant would be
//
//  1:  if not revision:
//  2:      Clone (Depth=1)                        // (A)
//  3:  else
//  4:      Clone (Depth=1,SingleBranch=revision)  // (B,C?)
//  5:      if ok: done                            // (B!)
//  7:      Clone ()                               // (C)
//  8:      hash = ResolveRevision (revision)
//  9:      Checkout (hash)
//
// I.e. try to use a revision as branch name first, to get the `SingleBranch`
// optimization.  When that fails fall back to regular cloning and checkout.  This fall
// back should happen only for (C).
//
// ## Decision
//
// Going with the second solution. While there is more complexity it is not that much
// more.  Note also that using a commit id (C) is considered unusual. Using a branch (B)
// is much more expected.

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-logr/logr"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/s3manager"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ImportGit handles the API endpoint /namespaces/:namespace/applications/:app/import-git.
// It receives a Git repo url and revision, clones that (shallow clone), creates a tarball
// of the repo and puts it on S3.
func ImportGit(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	name := c.Param("app")

	url := c.PostForm("giturl")
	revision := c.PostForm("gitrev")

	gitRepo, err := os.MkdirTemp("", "epinio-app")
	if err != nil {
		return apierror.InternalError(err, "can't create temp directory")
	}
	defer os.RemoveAll(gitRepo)

	// clone/fetch/checkout
	err = getRepository(ctx, log, gitRepo, url, revision)
	if err != nil {
		return apierror.InternalError(err,
			fmt.Sprintf("cloning the git repository: %s @ %s", url, revision))
	}

	// Create a tarball
	tmpDir, tarball, err := helpers.Tar(gitRepo)
	defer func() {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	}()
	if err != nil {
		return apierror.InternalError(err, "create a tarball from the git repository")
	}

	// Upload to S3
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}
	connectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster, helmchart.Namespace(), "epinio-s3-connection-details")
	if err != nil {
		return apierror.InternalError(err, "fetching the S3 connection details from the Kubernetes secret")
	}
	manager, err := s3manager.New(connectionDetails)
	if err != nil {
		return apierror.InternalError(err, "creating an S3 manager")
	}

	username := requestctx.User(ctx).Username
	blobUID, err := manager.Upload(ctx, tarball, map[string]string{
		"app": name, "namespace": namespace, "username": username,
	})
	if err != nil {
		return apierror.InternalError(err, "uploading the application sources blob")
	}
	log.Info("uploaded app", "namespace", namespace, "app", name, "blobUID", blobUID)

	// Return the id of the new blob
	response.OKReturn(c, models.ImportGitResponse{
		BlobUID: blobUID,
	})
	return nil
}

func getRepository(ctx context.Context, log logr.Logger, gitRepo, url, revision string) error {
	if revision == "" {
		// Input A: repository, no revision.
		log.Info("importgit, cloning simple", "url", url)
		_, err := shallowClone(ctx, gitRepo, url)
		return err
	}

	// Input B or C: Attempt to treat as B (revision is branch name)

	log.Info("importgit, cloning branch", "url", url, "revision", revision)
	_, err := branchClone(ctx, gitRepo, url, revision)
	if err == nil {
		// Was branch name, done.
		return nil
	}
	if !errors.Is(err, git.NoMatchingRefSpecError{}) || !plumbing.IsHash(revision) {
		// Some other error, or the revision does not look like a commit id (C)
		return err
	}

	// Attempt input C: revision might be commit id.
	// 2 stage process - A simple clone followed by a checkout

	log.Info("importgit, cloning simple, commit id", "url", url)
	repository, err := generalClone(ctx, gitRepo, url)
	if err != nil {
		return err
	}

	log.Info("importgit, resolve", "revision", revision)
	hash, err := repository.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return err
	}

	log.Info("importgit, resolved", "revision", hash)

	checkout, err := repository.Worktree()
	if err != nil {
		return err
	}

	log.Info("importgit, checking out", "url", url, "revision", hash)

	return checkout.Checkout(&git.CheckoutOptions{
		Hash:  *hash,
		Force: true,
	})
}

func branchClone(ctx context.Context, gitRepo, url, revision string) (*git.Repository, error) {
	// Note, it is shallow too
	return git.PlainCloneContext(ctx, gitRepo, false, &git.CloneOptions{
		URL:           url,
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(revision),
		Depth:         1,
	})
}

func shallowClone(ctx context.Context, gitRepo, url string) (*git.Repository, error) {
	return git.PlainCloneContext(ctx, gitRepo, false, &git.CloneOptions{
		URL:   url,
		Depth: 1,
	})
}

func generalClone(ctx context.Context, gitRepo, url string) (*git.Repository, error) {
	return git.PlainCloneContext(ctx, gitRepo, false, &git.CloneOptions{
		URL: url,
	})
}
