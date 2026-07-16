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

package gitproxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/auth"
	gitbridge "github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ProxyHandler is the gin.Handler for the Proxy func
func ProxyHandler(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	gitManager, err := gitbridge.NewManager(cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if err != nil {
		return apierror.InternalError(err, "creating git configuration manager")
	}

	return Proxy(c, gitManager)
}

// Proxy will proxy the URL provided in the GitProxyRequest
// eventually using the gitconfiguration spcified in the gitconfig.
func Proxy(c *gin.Context, gitManager *gitbridge.Manager) apierror.APIErrors {
	ctx := c.Request.Context()

	proxyRequest := &models.GitProxyRequest{}
	if err := c.BindJSON(proxyRequest); err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	if err := ValidateURL(proxyRequest.URL); err != nil {
		return apierror.NewBadRequestErrorf("invalid proxied URL: %s", err.Error())
	}

	// create request to proxy
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyRequest.URL, nil)
	if err != nil {
		return apierror.InternalError(err, "creating proxy request")
	}

	// If a git configuration was specified, resolve it and verify the user is
	// allowed to use it before its credentials are applied to the proxied request.
	var gitConfig *gitbridge.Configuration
	if proxyRequest.Gitconfig != "" {
		user := requestctx.User(ctx)

		config, found := gitManager.Configuration(proxyRequest.Gitconfig)
		if !found {
			return apierror.NewNotFoundError("gitconfig", proxyRequest.Gitconfig)
		}
		if !auth.CanUseGitconfig(user, *config) {
			return apierror.NewAPIError(
				fmt.Sprintf("user unauthorized to use gitconfig [%s]", proxyRequest.Gitconfig),
				http.StatusForbidden,
			)
		}
		// Bind the credential to its own instance: the proxied host must be one
		// this config is allowed to reach, so a token is never forwarded onward.
		if !config.AllowsHost(proxyRequest.URL) {
			return apierror.NewAPIError(
				fmt.Sprintf("gitconfig [%s] is not allowed for the requested host", proxyRequest.Gitconfig),
				http.StatusForbidden,
			)
		}

		gitConfig = config
	}

	// Apply the gitconfig credentials using the scheme the provider expects.
	// GitHub accepts HTTP Basic (username + token); GitLab's REST API does not,
	// it expects the token in the PRIVATE-TOKEN header.
	if gitConfig != nil && gitConfig.Password != "" {
		switch gitConfig.Provider {
		case models.ProviderGitlab, models.ProviderGitlabEnterprise:
			req.Header.Set("PRIVATE-TOKEN", gitConfig.Password)
		default:
			if gitConfig.Username != "" {
				req.SetBasicAuth(gitConfig.Username, gitConfig.Password)
			}
		}
	}

	client, err := getProxyClient(gitConfig)
	if err != nil {
		return apierror.InternalError(err, "creating proxy client")
	}

	err = doRequest(client, req, c.Writer)
	if err != nil {
		return apierror.InternalError(err, "proxying request")
	}

	return nil
}

// ValidateURL will validate the URL to proxy. We don't want to let the user proxy everything
// but only some of the Github or Gitlab APIs.
func ValidateURL(proxiedURL string) error {
	u, err := url.Parse(proxiedURL)
	if err != nil {
		return errors.Wrap(err, "parsing proxied URL")
	}

	// if the host is known (Github or Github Enterprise Cloud) just validate the path
	// https://docs.github.com/en/enterprise-cloud@latest/rest/enterprise-admin
	//
	// GitHub Enterprise Cloud with data residency serves the same API from a
	// per-tenant subdomain, api.<subdomain>.ghe.com, with the same bare paths as
	// api.github.com (no /api/v3 prefix). .ghe.com is GitHub-operated.
	if u.Host == "api.github.com" || isGHEComAPIHost(u.Host) {
		return validateGithubURL(u.EscapedPath())
	}

	// if the path starts with '/api/v3' we are assuming this to be a selfhosted Github Enterprise Server
	// https://docs.github.com/en/enterprise-server@3.10/rest/enterprise-admin
	if strings.HasPrefix(u.Path, "/api/v3") {
		path := strings.TrimPrefix(u.EscapedPath(), "/api/v3")
		return validateGithubURL(path)
	}

	// if the path starts with '/api/v4' we are assuming this to be a Gitlab server
	// https://docs.gitlab.com/ee/api/rest/
	if strings.HasPrefix(u.Path, "/api/v4") {
		path := strings.TrimPrefix(u.EscapedPath(), "/api/v4")
		return validateGitlabURL(path)
	}

	return fmt.Errorf("unknown URL '%s'", proxiedURL)
}

// isGHEComAPIHost reports whether host is a GitHub Enterprise Cloud data-residency
// REST API host, i.e. api.<subdomain>.ghe.com. `.ghe.com` is a GitHub-operated
// domain, so a token cannot be steered to an attacker-controlled host through it.
func isGHEComAPIHost(host string) bool {
	return strings.HasPrefix(host, "api.") && strings.HasSuffix(host, ".ghe.com")
}

// validateGithubURL will validate if the requested API is a whitelisted one.
// We don't want to let the user call all the Github APIs with the provided tokens.
// The supported APIs are:
// - /repos/USERNAME/REPO
// - /repos/USERNAME/REPO/commits
// - /repos/USERNAME/REPO/branches
// - /repos/USERNAME/REPO/branches/BRANCH
// - /users/USERNAME/repos
// - /users/USERNAME          (account info, used to detect user vs org)
// - /user/repos              (authenticated user's repos, including private)
// - /orgs/ORG/repos          (org repos, including private)
// - /search/repositories
func validateGithubURL(path string) error {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// with 2 parts we support these endpoints:
	// - /search/repositories
	// - /user/repos
	// - /users/USERNAME
	if len(parts) == 2 {
		if parts[0] == "search" && parts[1] == "repositories" {
			return nil
		}
		if parts[0] == "user" && parts[1] == "repos" {
			return nil
		}
		if parts[0] == "users" {
			return nil
		}
	}

	// with 3 parts we support these endpoints:
	// - /repos/USERNAME/REPO
	// - /users/USERNAME/repos
	// - /orgs/ORG/repos
	if len(parts) == 3 {
		if parts[0] == "repos" ||
			(parts[0] == "users" && parts[2] == "repos") ||
			(parts[0] == "orgs" && parts[2] == "repos") {
			return nil
		}
	}

	// with 4 parts we support these endpoints:
	// - /repos/USERNAME/REPO/commits
	// - /repos/USERNAME/REPO/branches
	if len(parts) == 4 {
		if parts[0] == "repos" && (parts[3] == "commits" || parts[3] == "branches") {
			return nil
		}
	}

	// with 5 parts we support this endpoint:
	// - /repos/USERNAME/REPO/branches/BRANCH
	if len(parts) == 5 {
		if parts[0] == "repos" && parts[3] == "branches" {
			return nil
		}
	}

	return fmt.Errorf("invalid Github URL: '%s'", path)
}

// validateGitlabURL will validate if the requested API is a whitelisted one.
// We don't want to let the user call all the Gitlab APIs with the provided
// tokens.
// Gitlab use the project ID or the url encoded "USERNAME/REPO" string, hence
// we are checking for the second one.
//
// The supported APIs are:
// - /avatar
// - /projects                (e.g. ?membership=true, the user's projects including private)
// - /projects/USERNAME%2FREPO
// - /users                   (e.g. ?username=NAME, to resolve a user)
// - /users/USERNAME/projects
// - /groups/GROUP            (group info, used to detect user vs group)
// - /groups/USERNAME/projects
// - /projects/USERNAME%2FREPO/repository/branches
// - /projects/USERNAME%2FREPO/repository/commits
// - /projects/REPO/repository/branches/BRANCH
//
// Project listing/search uses the `?search=` query on the list endpoints; GitLab
// has no `/search/repositories` endpoint (that is GitHub-shaped).
func validateGitlabURL(path string) error {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// with 1 part we support these endpoints:
	// - /avatar
	// - /projects        (e.g. ?membership=true, the user's projects including private)
	// - /users           (e.g. ?username=NAME, to resolve a user / detect namespace)
	if len(parts) == 1 {
		if parts[0] == "avatar" || parts[0] == "projects" || parts[0] == "users" {
			return nil
		}
	}

	// with 2 parts we support these endpoints:
	// - /projects/USERNAME%2FREPO
	// - /groups/GROUP     (group info, used to detect user vs group)
	if len(parts) == 2 {
		if parts[0] == "projects" || parts[0] == "groups" {
			return nil
		}
	}

	// with 3 parts we support these endpoints:
	// - /users/USERNAME/projects
	// - /groups/USERNAME/projects
	if len(parts) == 3 {
		if (parts[0] == "users" || parts[0] == "groups") && parts[2] == "projects" {
			return nil
		}
	}

	// with 4 parts we support these endpoints:
	// - /projects/USERNAME%2FREPO/repository/branches
	// - /projects/USERNAME%2FREPO/repository/commits
	if len(parts) == 4 {
		if parts[0] == "projects" && parts[2] == "repository" &&
			(parts[3] == "branches" || parts[3] == "commits") {
			return nil
		}
	}

	// with 5 parts we support this endpoint:
	// - /projects/REPO/repository/branches/BRANCH
	if len(parts) == 5 {
		if parts[0] == "projects" && parts[2] == "repository" && parts[3] == "branches" {
			return nil
		}
	}

	return fmt.Errorf("invalid Gitlab URL '%s'", path)
}

// getProxyClient will create a *http.Client based on the provided *gitbridge.Configuration.
// For example it will skip the SSL verification or load the provided certificates.
func getProxyClient(gitConfig *gitbridge.Configuration) (*http.Client, error) {
	client := &http.Client{}

	if gitConfig == nil {
		return client, nil
	}

	// Re-check the host binding on every redirect hop. Go drops the Authorization
	// header on a cross-host redirect, but not a custom header such as GitLab's
	// PRIVATE-TOKEN, so without this a redirect could forward the token to an
	// unrelated host. Refusing the redirect stops the credential from leaving the
	// configured instance.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if !gitConfig.AllowsHost(req.URL.String()) {
			return fmt.Errorf("redirect to disallowed host %q blocked", req.URL.Host)
		}
		return nil
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	tlsConfig.InsecureSkipVerify = gitConfig.SkipSSL

	// add custom certs
	// https://github.com/go-git/go-git/blob/0377d0627fa4e32a576634f441e72153807e395a/plumbing/transport/http/common.go#L187-L201

	if len(gitConfig.Certificate) > 0 {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return nil, errors.Wrap(err, "loading SystemCertPool")
		}
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		rootCAs.AppendCertsFromPEM(gitConfig.Certificate)

		tlsConfig.RootCAs = rootCAs
	}

	transport.TLSClientConfig = tlsConfig
	client.Transport = transport

	return client, nil
}

// doRequest will execute the proxied request copying the response and the headers in the ResponseWriter
func doRequest(client *http.Client, req *http.Request, writer http.ResponseWriter) error {
	resp, err := client.Do(req) // nolint:gosec // git proxy, URL from request validated by caller
	if err != nil {
		return errors.Wrap(err, "executing proxied request")
	}

	writer.WriteHeader(resp.StatusCode)

	for k, values := range resp.Header {
		for _, v := range values {
			writer.Header().Add(k, v)
		}
	}

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return errors.Wrap(err, "copying proxied response")
	}

	return nil
}
