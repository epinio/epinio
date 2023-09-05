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
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	gitbridge "github.com/epinio/epinio/internal/bridge/git"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

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

func Proxy(c *gin.Context, gitManager *gitbridge.Manager) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx)

	proxyRequest := &models.GitProxyRequest{}
	if err := c.BindJSON(proxyRequest); err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	if _, err := url.Parse(proxyRequest.URL); err != nil {
		return apierror.NewBadRequestError("invalid proxied URL")
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
