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
//   - (C) repository, commit id (short/long), tags or any supported ref
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
// Finding the matching reference for the specified revision it's a "complex" operation, and it's done only with the last option.
// With option A and B we already know about the matching branch, and we can early return
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
	"github.com/go-git/go-git/v5/plumbing/object"
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
	ref, err := checkoutRepository(ctx, log, gitRepo, url, revision)
	if err != nil {
		return apierror.InternalError(err,
			fmt.Sprintf("cloning the git repository: %s @ %s", url, revision))
	}

	var branch string
	if ref != nil {
		branch = ref.Name().Short()
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
		Branch:  branch,
	})
	return nil
}

var (
	errDone              = errors.New("iteration done")
	errReferenceNotFound = errors.New("reference not found")
)

// checkoutRepository will clone the repository and it will checkout the revision
// It will also try to find the matching branch/reference, and if found this will be returned
func checkoutRepository(ctx context.Context, log logr.Logger, gitRepo, url, revision string) (*plumbing.Reference, error) {
	if revision == "" {
		// Input A: repository, no revision.
		log.Info("importgit, cloning simple", "url", url)
		return shallowCheckout(ctx, gitRepo, url)
	}

	ref, err := branchCheckout(ctx, gitRepo, url, revision)
	// it was a branch, and everything went fine
	if err == nil {
		return ref, nil
	}
	// some other error occurred
	if !errors.Is(err, git.NoMatchingRefSpecError{}) {
		return nil, err
	}

	// we are left we the full clone option
	log.Info("importgit, cloning plain", "url", url)
	repo, err := git.PlainCloneContext(ctx, gitRepo, false, &git.CloneOptions{URL: url})
	if err != nil {
		return nil, err
	}

	log.Info("importgit, resolve", "revision", revision)
	hash, err := repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, err
	}
	log.Info("importgit, resolved", "revision", hash)

	ref, err = findReferenceForRevision(repo, *hash)
	if err != nil && !errors.Is(err, errReferenceNotFound) {
		return nil, err
	}

	checkout, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	log.Info("importgit, checking out", "url", url, "revision", hash)

	err = checkout.Checkout(&git.CheckoutOptions{
		Hash:  *hash,
		Force: true,
	})
	if err != nil {
		return nil, err
	}

	return ref, nil
}

func shallowCheckout(ctx context.Context, gitRepo, url string) (*plumbing.Reference, error) {
	repo, err := git.PlainCloneContext(ctx, gitRepo, false, &git.CloneOptions{
		URL:   url,
		Depth: 1,
	})
	if err != nil {
		return nil, err
	}

	return repo.Head()
}

func branchCheckout(ctx context.Context, gitRepo, url, revision string) (*plumbing.Reference, error) {
	repo, err := git.PlainCloneContext(ctx, gitRepo, false, &git.CloneOptions{
		URL:           url,
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(revision),
		Depth:         1,
	})
	if err == nil {
		return repo.Head()
	}

	return nil, err
}

// findReferenceForRevision will loop through all the available refs (branches, tags, ...) and it will try
// to see if any of those contains the specified revision.
func findReferenceForRevision(repo *git.Repository, revision plumbing.Hash) (*plumbing.Reference, error) {
	// this map will be used to stop the iteration when we have reached already seen commits
	commitMap := map[string]struct{}{}

	w, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	refIter, err := repo.References()
	if err != nil {
		return nil, err
	}

	var matchingRef *plumbing.Reference

	// we are going to loop on every refs and check if one of them contain the revision
	err = refIter.ForEach(func(r *plumbing.Reference) error {
		err = w.Checkout(&git.CheckoutOptions{
			Branch: r.Name(),
			Force:  true,
		})
		if err != nil {
			return err
		}

		found, err := containsRevision(repo, revision, commitMap)
		if err != nil {
			return err
		}
		if found {
			matchingRef = r
			return errDone
		}

		return nil
	})
	// if something bad happened, return
	if err != nil && !errors.Is(err, errDone) {
		return nil, err
	}
	// no matching reference found, return a specific error
	if matchingRef == nil {
		return nil, errReferenceNotFound
	}

	return matchingRef, nil
}

// containsRevision will look for all the commits in the current repo to check for the revision.
// It will look only in the current working tree, and it will return an errDone when the iteration was completed.
// The passed commitMap is used to stop when we have reached an already checked commit, so we don't need to look back to the previous history.
func containsRevision(repo *git.Repository, revision plumbing.Hash, commitMap map[string]struct{}) (bool, error) {
	var found bool

	commitIter, err := repo.Log(&git.LogOptions{Order: git.LogOrderCommitterTime})
	if err != nil {
		return found, err
	}

	err = commitIter.ForEach(func(c *object.Commit) error {
		if c.Hash.String() == revision.String() {
			found = true
			return errDone
		}

		if _, found := commitMap[c.Hash.String()]; found {
			return errDone
		}

		commitMap[c.Hash.String()] = struct{}{}
		return nil
	})

	if err != nil && !errors.Is(err, errDone) {
		return found, err

	}
	return found, nil
}
