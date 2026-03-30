// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
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
//	 - (A) repository, no revision (empty string)
//	 - (B) repository, branch name
//	 - (C) repository, commit id (short/long), tags or any supported ref
//
// ## Considerations:
//
//	 - Correctness
//	 - Cloning performance
//
// The go-git cloning function has two attributes influencing performance
//
//	 - `Depth` specifies the depth towards which to clone.
//	 - `SingleBranch` specifies the sole branch to check out.
//
// The second flag comes with a problem. Using it __demands__ a branch.  And whatever is
// found in the `ReferenceName` of the CloneOptions is used.	Even if it is an empty
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
// Clone (Depth=1)
// if revision:
// 	hash = ResolveRevision (revision)
//	Checkout (hash)
//
// with no `SingleBranch` in sight, just `Depth`.
//
// More complex, hopefully more performant would be
//
//	1: if not revision:
//	2: Clone (Depth=1)	// (A)
//	3: else
//	4: Clone (Depth=1,SingleBranch=revision)  // (B,C?)
//	5: if ok: done // (B!)
//	7: Clone () // (C)
//	8: hash = ResolveRevision (revision)
//	9: Checkout (hash)
//
// I.e. try to use a revision as branch name first, to get the `SingleBranch`
// optimization.	When that fails fall back to regular cloning and checkout.	This fall
// back should happen only for (C).
//
// Finding the matching reference for the specified revision it's a "complex" operation, and it's done only with the last option.
// With option A and B we already know about the matching branch, and we can early return
//
// ## Decision
//
// Going with the second solution. While there is more complexity it is not that much
// more.	Note also that using a commit id (C) is considered unusual. Using a branch (B)
// is much more expected.

import (
	"context"
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	gitbridge "github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/s3manager"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"golang.org/x/crypto/ssh"

	"github.com/skeema/knownhosts"
)

// ImportGit handles the API endpoint /namespaces/:namespace/applications/:app/import-git.
// It receives a Git repo url and revision, clones that (shallow clone), creates a tarball
// of the repo and puts it on S3.
func ImportGit(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	name := c.Param("app")

	giturl := c.PostForm("giturl")
	revision := c.PostForm("gitrev")

	errGitURL := validateGitURL(giturl)
	if errGitURL != nil {
		return errGitURL
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	username := requestctx.User(ctx).Username
	result, apiErr := ImportGitRun(ctx, cluster, namespace, name, giturl, revision, username)
	if apiErr != nil {
		return apiErr
	}

	response.OKReturn(c, result)
	return nil
}

// ImportGitRun clones the repository, uploads sources to blob storage, and returns blob metadata.
func ImportGitRun(ctx context.Context, cluster *kubernetes.Cluster, namespace, name, giturl, revision, username string) (models.ImportGitResponse, apierror.APIErrors) {
	log := requestctx.Logger(ctx)

	gitManager, err := gitbridge.NewManager(cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if err != nil {
		return models.ImportGitResponse{}, apierror.InternalError(err, "creating git configuration manager")
	}

	gitRepo, err := os.MkdirTemp("", "epinio-app")
	if err != nil {
		return models.ImportGitResponse{}, apierror.InternalError(err, "can't create temp directory")
	}

	defer func() {
		if err := os.RemoveAll(gitRepo); err != nil {
			log.Errorw("failed to remove git repo", "error", err)
		}
	}()
	gitConfig, err := gitManager.FindConfiguration(giturl)
	if err != nil {
		errMsg := fmt.Sprintf("finding git configuration for gitURL [%s]", giturl)
		return models.ImportGitResponse{}, apierror.InternalError(err, errMsg)
	}

	if gitConfig != nil {
		log.Infow("loaded git config",
			"giturl", giturl,
			"gitConfig", gitConfig.ID,
			"provider", gitConfig.Provider,
			"userOrg", gitConfig.UserOrg,
			"repository", gitConfig.Repository,
			"ssh_private_key_configured", len(gitConfig.PrivateKey) > 0,
		)
	} else {
		log.Infow("git config not found for giturl",
			"giturl", giturl,
			"revision", revision,
		)
	}

	// clone/fetch/checkout
	ref, err := checkoutRepository(ctx, gitRepo, giturl, revision, gitConfig)
	if err != nil {
		errMsg := fmt.Sprintf("cloning the git repository: %s @ %s", giturl, revision)
		return models.ImportGitResponse{}, apierror.InternalError(err, errMsg)
	}

	var branch string
	if ref != nil {
		branch = ref.Name().Short()
		revision = ref.Hash().String()
		log.Infow("resolved branch and revision", "branch", branch, "revision", revision)
	}

	// Create a tarball
	tmpDir, tarball, err := helpers.Tar(gitRepo, nil)
	defer func() {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	}()
	if err != nil {
		return models.ImportGitResponse{}, apierror.InternalError(err, "create a tarball from the git repository")
	}

	// Upload to S3
	connectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster, helmchart.Namespace(), "epinio-s3-connection-details")
	if err != nil {
		return models.ImportGitResponse{}, apierror.InternalError(err, "fetching the S3 connection details from the Kubernetes secret")
	}
	manager, err := s3manager.New(connectionDetails)
	if err != nil {
		return models.ImportGitResponse{}, apierror.InternalError(err, "creating an S3 manager")
	}

	blobUID, err := manager.Upload(ctx, tarball, map[string]string{
		"app": name, "namespace": namespace, "username": username,
	})
	if err != nil {
		// Check if the error is due to quota exhaustion
		if s3manager.IsQuotaExceededError(err) {
			return models.ImportGitResponse{}, apierror.NewQuotaExceededError("",
				"uploading the application sources blob: "+err.Error())
		}
		return models.ImportGitResponse{}, apierror.InternalError(err, "uploading the application sources blob")
	}

	log.Infow("uploaded app", "namespace", namespace, "app", name, "blobUID", blobUID)

	return models.ImportGitResponse{
		BlobUID:  blobUID,
		Branch:   branch,
		Revision: revision,
	}, nil
}

func validateGitURL(gitURL string) apierror.APIErrors {
	if gitURL == "" {
		return apierror.NewBadRequestError("missing giturl")
	}

	if strings.HasPrefix(gitURL, "git@") {
		if err := gitbridge.ValidateRepoURL(gitURL); err != nil {
			return apierror.NewBadRequestErrorf("invalid giturl: %s", err.Error())
		}
		return nil
	}

	u, err := url.Parse(gitURL)
	if err != nil {
		return apierror.NewBadRequestErrorf("invalid giturl %s", err.Error())
	}

	if u.Scheme == "" || u.Host == "" {
		return apierror.NewBadRequestErrorf("missing scheme or host in giturl [%s://%s]", u.Scheme, u.Host)
	}

	return nil
}

var (
	errReferenceNotFound = errors.New("reference not found")
)

// checkoutRepository will clone the repository and it will checkout the revision
// It will also try to find the matching branch/reference, and if found this will be returned
func checkoutRepository(ctx context.Context, gitRepo, url, revision string, gitconfig *gitbridge.Configuration) (*plumbing.Reference, error) {
	log := requestctx.Logger(ctx)
	cloneOptions, knownHostsTempDir, err := configureCloneOptions(ctx, git.CloneOptions{}, url, gitconfig)
	if err != nil {
		return nil, err
	}
	if knownHostsTempDir != "" {
		defer func() {
			_ = os.RemoveAll(knownHostsTempDir)
		}()
	}

	if revision == "" {
		// Input A: repository, no revision.
		log.Infow("importgit, cloning simple", "url", url)
		return shallowCheckout(ctx, gitRepo, cloneOptions)
	}

	ref, err := branchCheckout(ctx, gitRepo, revision, cloneOptions)
	// it was a branch, and everything went fine
	if err == nil {
		return ref, nil
	}
	// some other error occurred
	if !errors.Is(err, git.NoMatchingRefSpecError{}) {
		return nil, err
	}

	// we are left we the full clone option
	log.Infow("importgit, cloning plain", "url", url)
	repo, err := git.PlainCloneContext(ctx, gitRepo, false, &cloneOptions)
	if err != nil {
		return nil, err
	}

	log.Infow("importgit, resolve", "revision", revision)
	hash, err := repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, err
	}
	log.Infow("importgit, resolved", "revision", hash)

	ref, err = findReferenceForRevision(repo, *hash)
	if err != nil && !errors.Is(err, errReferenceNotFound) {
		return nil, err
	}

	checkout, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	log.Infow("importgit, checking out", "url", url, "revision", hash)

	err = checkout.Checkout(&git.CheckoutOptions{
		Hash:  *hash,
		Force: true,
	})
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// insecurePublicKeys is a go-git SSH AuthMethod implementation that disables host key verification
// without requiring any known_hosts file to exist.
//
// go-git still tries to load known_hosts when HostKeyAlgorithms is empty, so we provide a small
// explicit set of common algorithms.
type insecurePublicKeys struct {
	user   string
	signer ssh.Signer
}

func (a *insecurePublicKeys) Name() string {
	return "ssh-public-keys-insecure"
}

func (a *insecurePublicKeys) String() string {
	return fmt.Sprintf("user: %s, name: %s", a.user, a.Name())
}

func (a *insecurePublicKeys) ClientConfig() (*ssh.ClientConfig, error) {
	return &ssh.ClientConfig{
		User: a.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(a.signer),
		},
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{"ssh-ed25519", "ecdsa-sha2-nistp256", "ssh-rsa"},
	}, nil
}

// knownHostsPublicKeys is a go-git SSH AuthMethod that uses a host-key DB generated at runtime.
// This avoids relying on any external/ambient ~/.ssh/known_hosts.
type knownHostsPublicKeys struct {
	user             string
	signer          ssh.Signer
	hostWithPort    string
	hostKeyCallback ssh.HostKeyCallback
	hostKeyAlgos    []string
}

func (a *knownHostsPublicKeys) Name() string {
	return "ssh-public-keys-knownhosts"
}

func (a *knownHostsPublicKeys) String() string {
	return fmt.Sprintf("user: %s, name: %s", a.user, a.Name())
}

func (a *knownHostsPublicKeys) ClientConfig() (*ssh.ClientConfig, error) {
	return &ssh.ClientConfig{
		User: a.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(a.signer),
		},
		HostKeyCallback:   a.hostKeyCallback,
		HostKeyAlgorithms: a.hostKeyAlgos,
	}, nil
}

func gitHostFromGitURL(gitURL string) (string, error) {
	if strings.HasPrefix(gitURL, "git@") {
		// git@github.com:org/repo.git
		rest := strings.TrimPrefix(gitURL, "git@")
		idx := strings.Index(rest, ":")
		if idx < 0 {
			return "", fmt.Errorf("invalid scp-style git URL [%s]", gitURL)
		}
		return rest[:idx], nil
	}
	u, err := url.Parse(gitURL)
	if err != nil {
		return "", err
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("missing hostname in git url [%s]", gitURL)
	}
	return u.Hostname(), nil
}

func tryBuildKnownHostsForHost(host string) (tempDir string, hostKeyCallback ssh.HostKeyCallback, hostKeyAlgos []string, ok bool) {
	keyscanPath, err := exec.LookPath("ssh-keyscan")
	if err != nil {
		return "", nil, nil, false
	}

	tmpDir, err := os.MkdirTemp("", "epinio-knownhosts-")
	if err != nil {
		return "", nil, nil, false
	}

	knownHostsPath := filepath.Join(tmpDir, "known_hosts")
	// Use the key types that cover GitHub/GitLab in practice.
	// ssh-keyscan -t supports multiple -t.
	cmd := exec.Command(
		keyscanPath,
		"-t", "rsa",
		"-t", "ed25519",
		host,
	)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, nil, false
	}

	if strings.TrimSpace(out.String()) == "" {
		_ = os.RemoveAll(tmpDir)
		return "", nil, nil, false
	}

	// Persist known_hosts for parsing.
	if err := os.WriteFile(knownHostsPath, out.Bytes(), 0600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, nil, false
	}

	hkdb, err := knownhosts.NewDB(knownHostsPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, nil, false
	}

	hostWithPort := fmt.Sprintf("%s:22", host)
	algos := hkdb.HostKeyAlgorithms(hostWithPort)
	if len(algos) == 0 {
		_ = os.RemoveAll(tmpDir)
		return "", nil, nil, false
	}

	return tmpDir, hkdb.HostKeyCallback(), algos, true
}

// configureCloneOptions sets URL, transport, and auth. Uses SSH when a private key is configured.
func configureCloneOptions(ctx context.Context, opts git.CloneOptions, requestedURL string, config *gitbridge.Configuration) (git.CloneOptions, string, error) {
	log := requestctx.Logger(ctx)
	if config != nil && len(config.PrivateKey) > 0 {
		opts.URL = toSSHCloneURL(requestedURL)
		signer, err := ssh.ParsePrivateKey(bytes.TrimSpace(config.PrivateKey))
		if err != nil {
			return opts, "", err
		}

		// Prefer generating a known_hosts for the host (if ssh-keyscan exists).
		// This mirrors your working local `main.go` approach and avoids go-git needing any ambient files.
		host, err := gitHostFromGitURL(requestedURL)
		if err == nil && host != "" {
			tempDir, cb, algos, ok := tryBuildKnownHostsForHost(host)
			if ok && cb != nil && len(algos) > 0 {
				opts.Auth = &knownHostsPublicKeys{
					user:             "git",
					signer:          signer,
					hostWithPort:    fmt.Sprintf("%s:22", host),
					hostKeyCallback: cb,
					hostKeyAlgos:    algos,
				}
				log.Infow("importgit, ssh clone with private key; using runtime known_hosts via ssh-keyscan",
					"requestedURL", requestedURL,
					"cloneURL", opts.URL,
					"host", host,
				)
				return opts, tempDir, nil
			}
		}

		// Fallback: disable host verification but explicitly set algorithms so go-git doesn't try to load known_hosts files.
		opts.Auth = &insecurePublicKeys{
			user:   "git",
			signer: signer,
		}
		log.Infow("importgit, ssh clone with private key; host key verification disabled (no known_hosts dependency)",
			"requestedURL", requestedURL,
			"cloneURL", opts.URL,
			"ssh_private_key_configured", len(config.PrivateKey) > 0,
		)
		return opts, "", nil
	}

	opts.URL = requestedURL
	if config == nil {
		return opts, "", nil
	}

	if len(config.PrivateKey) == 0 {
		log.Infow("importgit, clone without ssh private key",
			"requestedURL", requestedURL,
			"gitConfig", config.ID,
			"provider", config.Provider,
		)
	}

	opts.InsecureSkipTLS = config.SkipSSL

	if config.Username != "" && config.Password != "" {
		opts.Auth = &http.BasicAuth{
			Username: config.Username,
			Password: config.Password,
		}
	}

	if len(config.Certificate) > 0 {
		opts.CABundle = config.Certificate
	}

	return opts, "", nil
}

func toSSHCloneURL(gitURL string) string {
	if strings.HasPrefix(gitURL, "git@") {
		return gitURL
	}
	u, err := url.Parse(gitURL)
	if err != nil {
		return gitURL
	}
	path := strings.TrimPrefix(strings.TrimSuffix(u.Path, ".git"), "/")
	if path == "" {
		return gitURL
	}
	return fmt.Sprintf("git@%s:%s", u.Host, path)
}

func shallowCheckout(ctx context.Context, gitRepo string, opts git.CloneOptions) (*plumbing.Reference, error) {
	opts.Depth = 1

	repo, err := git.PlainCloneContext(ctx, gitRepo, false, &opts)
	if err != nil {
		return nil, err
	}

	return repo.Head()
}

func branchCheckout(ctx context.Context, gitRepo, revision string, opts git.CloneOptions) (*plumbing.Reference, error) {
	opts.Depth = 1
	opts.SingleBranch = true
	opts.ReferenceName = plumbing.NewBranchReferenceName(revision)

	repo, err := git.PlainCloneContext(ctx, gitRepo, false, &opts)
	if err != nil {
		return nil, err
	}

	return repo.Head()
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
			return storer.ErrStop
		}

		return nil
	})
	// if something bad happened, return
	if err != nil && !errors.Is(err, storer.ErrStop) {
		return nil, err
	}
	// no matching reference found, return a specific error
	if matchingRef == nil {
		return nil, errReferenceNotFound
	}

	// we need to create a new reference from the one matching the revision,
	// because it will not return the expected commit that we checked, but the last one.
	// We also need to remove the 'origin/' prefix, or the UI will not work.
	refName := strings.TrimPrefix(matchingRef.Name().Short(), "origin/")
	matchingRef = plumbing.NewReferenceFromStrings(refName, revision.String())
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
			return storer.ErrStop
		}

		if _, found := commitMap[c.Hash.String()]; found {
			return storer.ErrStop
		}

		commitMap[c.Hash.String()] = struct{}{}
		return nil
	})

	if err != nil && !errors.Is(err, storer.ErrStop) {
		return found, err

	}
	return found, nil
}
