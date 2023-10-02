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

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	gitbridge "github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ProxyHandler is the gin.Handler for the Proxy func
func ProxyHandler(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	gitManager, err := gitbridge.NewManager(logger, cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if err != nil {
		return apierror.InternalError(err, "creating git configuration manager")
	}

	return Proxy(c, gitManager)
}

// Proxy will proxy the URL provided in the GitProxyRequest
// eventually using the gitconfiguration spcified in the gitconfig.
func Proxy(c *gin.Context, gitManager *gitbridge.Manager) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx)

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

	// if specified look for the git configuration
	var gitConfig *gitbridge.Configuration
	if proxyRequest.Gitconfig != "" {
		for i := range gitManager.Configurations {
			if proxyRequest.Gitconfig == gitManager.Configurations[i].ID {
				gitConfig = &gitManager.Configurations[i]
				break
			}
		}
		if gitConfig == nil {
			logger.Info("gitconfig not found", "id", proxyRequest.Gitconfig)
		}
	}

	if gitConfig != nil {
		// check BasicAuth
		if gitConfig.Username != "" && gitConfig.Password != "" {
			req.SetBasicAuth(gitConfig.Username, gitConfig.Password)
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
	if u.Host == "api.github.com" {
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

// validateGithubURL will validate if the requested API is a whitelisted one.
// We don't want to let the user call all the Github APIs with the provided tokens.
// The supported APIs are:
// - /repos/USERNAME/REPO
// - /repos/USERNAME/REPO/commits
// - /repos/USERNAME/REPO/branches
// - /repos/USERNAME/REPO/branches/BRANCH
// - /users/USERNAME/repos
// - /search/repositories
func validateGithubURL(path string) error {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// with 2 parts we support this endpoint:
	// - /search/repositories
	if len(parts) == 2 {
		if parts[0] == "search" && parts[1] == "repositories" {
			return nil
		}
	}

	// with 3 parts we support these endpoints:
	// - /repos/USERNAME/REPO
	// - /users/USERNAME/repos
	if len(parts) == 3 {
		if parts[0] == "repos" ||
			(parts[0] == "users" && parts[2] == "repos") {
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
// We don't want to let the user call all the Gitlab APIs with the provided tokens.
// Gitlab use the project ID or the url encoded "USERNAME/REPO" string, hence we are checking for the second one.
// The supported APIs are:
// - /avatar
// - /search/repositories
// - /projects/USERNAME%2FREPO
// - /users/USERNAME/projects
// - /groups/USERNAME/projects
// - /projects/USERNAME%2FREPO/repository/branches
// - /projects/USERNAME%2FREPO/repository/commits
// - /projects/REPO/repository/branches/BRANCH
func validateGitlabURL(path string) error {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// with 1 part we support this endpoint:
	// - /avatar
	if len(parts) == 1 {
		if parts[0] == "avatar" {
			return nil
		}
	}

	// with 2 parts we support these endpoints:
	// - /search/repositories
	// - /projects/USERNAME%2FREPO
	if len(parts) == 2 {
		if path == "/search/repositories" || parts[0] == "projects" {
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

	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if gitConfig != nil {
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
	}

	transport.TLSClientConfig = tlsConfig
	client.Transport = transport

	return client, nil
}

// doRequest will execute the proxied request copying the response and the headers in the ResponseWriter
func doRequest(client *http.Client, req *http.Request, writer http.ResponseWriter) error {
	resp, err := client.Do(req)
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
