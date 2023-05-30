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

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/helpers/termui"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/version"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"golang.org/x/oauth2"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

type responseError struct {
	error
	statusCode int
}

func (re *responseError) Unwrap() error   { return re.error }
func (re *responseError) StatusCode() int { return re.statusCode }

func wrapResponseError(err error, code int) *responseError {
	return &responseError{error: err, statusCode: code}
}

func (c *Client) DisableVersionWarning() {
	c.noVersionWarning = true
}

// VersionWarningEnabled returns true if versionWarning field is either not
// set of true. That makes "true" the default value unless DisableVersionWarning
// is called.
func (c *Client) VersionWarningEnabled() bool {
	return !c.noVersionWarning
}

func Get[T any](c *Client, endpoint string, v T) (T, error) {
	data, err := c.get(endpoint)
	if err != nil {
		return v, err
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return v, err
	}

	c.log.V(1).Info("response decoded", "response", v)

	return v, nil
}

func (c *Client) get(endpoint string) ([]byte, error) {
	return c.do(endpoint, "GET", "")
}

func (c *Client) post(endpoint string, data string) ([]byte, error) {
	return c.do(endpoint, "POST", data)
}

func (c *Client) patch(endpoint string, data string) ([]byte, error) {
	return c.do(endpoint, "PATCH", data)
}

func (c *Client) delete(endpoint string) ([]byte, error) {
	return c.do(endpoint, "DELETE", "")
}

// upload the given path as param "file" in a multipart form
func (c *Client) upload(endpoint string, path string) ([]byte, error) {
	uri := fmt.Sprintf("%s%s/%s", c.Settings.API, api.Root, endpoint)

	// open the tarball
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open tarball")
	}
	defer file.Close()

	// create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create multiform part")
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write to multiform part")
	}

	err = writer.Close()
	if err != nil {
		return nil, errors.Wrap(err, "failed to close multiform")
	}

	// make the request
	request, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build request")
	}

	request.Header.Add("Content-Type", writer.FormDataContentType())

	err = c.handleAuthorization(request)
	if err != nil {
		return []byte{}, err
	}

	response, err := c.HttpClient.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to POST to upload")
	}
	defer response.Body.Close()

	bodyBytes, _ := io.ReadAll(response.Body)
	if response.StatusCode == http.StatusCreated {
		return bodyBytes, nil
	}

	if response.StatusCode != http.StatusOK {
		return nil, wrapResponseError(fmt.Errorf("server status code: %s\n%s",
			http.StatusText(response.StatusCode), string(bodyBytes)),
			response.StatusCode)
	}

	// object was not created, but status was ok?
	return bodyBytes, nil
}

func (c *Client) do(endpoint, method, requestBody string) ([]byte, error) {
	uri := fmt.Sprintf("%s%s/%s", c.Settings.API, api.Root, endpoint)
	c.log.Info(fmt.Sprintf("%s %s", method, uri))

	reqLog := requestLogger(c.log, method, uri, requestBody)

	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		reqLog.V(1).Error(err, "cannot build request")
		return []byte{}, err
	}

	err = c.handleAuthorization(request)
	if err != nil {
		return []byte{}, err
	}

	response, err := c.HttpClient.Do(request)
	if err != nil {
		reqLog.V(1).Error(err, "request failed")
		castedErr, ok := err.(*url.Error)
		if !ok {
			return []byte{}, errors.New("couldn't cast request Error!")
		}
		if castedErr.Timeout() {
			return []byte{}, errors.New("request cancelled or timed out")
		}

		return []byte{}, errors.Wrap(err, "making the request")
	}
	defer response.Body.Close()
	reqLog.V(1).Info("request finished")

	serverVersion := response.Header.Get(api.VersionHeader)
	if c.VersionWarningEnabled() && serverVersion != "" {
		c.warnAboutVersionMismatch(serverVersion)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	respLog := responseLogger(c.log, response, string(bodyBytes))
	if err != nil {
		respLog.V(1).Error(err, "failed to read response body")
		return []byte{}, wrapResponseError(err, response.StatusCode)
	}

	respLog.V(1).Info("response received")

	if response.StatusCode == http.StatusCreated {
		return bodyBytes, nil
	}

	// TODO why is != 200 an error? there are valid codes in the 2xx, 3xx range
	if response.StatusCode != http.StatusOK {
		err := formatError(bodyBytes, response)

		if respLog.V(5).Enabled() {
			respLog = respLog.WithValues("body", string(bodyBytes))
		}
		respLog.V(1).Info("response is not StatusOK: " + err.Error())

		return bodyBytes, wrapResponseError(err, response.StatusCode)
	}

	return bodyBytes, nil
}

type ErrorFunc = func(response *http.Response, bodyBytes []byte, err error) error

// doWithCustomErrorHandling has a special handler for "response type" errors.
// These are errors where the server send a valid http response, but the status
// code is not 200.
// The ErrorFunc allows us to inspect the response, even unmarshal it into an
// api.ErrorResponse and change the returned error.
// Note: it's only used by ConfigurationDelete and that could be changed to transmit
// it's data in a normal Response, instead of an error?
func (c *Client) doWithCustomErrorHandling(endpoint, method, requestBody string, f ErrorFunc) ([]byte, error) {

	uri := fmt.Sprintf("%s%s/%s", c.Settings.API, api.Root, endpoint)
	c.log.Info(fmt.Sprintf("%s %s", method, uri))

	reqLog := requestLogger(c.log, method, uri, requestBody)

	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		reqLog.V(1).Error(err, "cannot build request")
		return []byte{}, err
	}

	err = c.handleAuthorization(request)
	if err != nil {
		return []byte{}, err
	}

	response, err := c.HttpClient.Do(request)
	if err != nil {
		reqLog.V(1).Error(err, "request failed")
		return []byte{}, err
	}
	defer response.Body.Close()
	reqLog.V(1).Info("request finished")

	bodyBytes, err := io.ReadAll(response.Body)
	respLog := responseLogger(c.log, response, string(bodyBytes))
	if err != nil {
		respLog.V(1).Error(err, "failed to read response body")
		return []byte{}, wrapResponseError(err, response.StatusCode)
	}

	respLog.V(1).Info("response received")

	if response.StatusCode == http.StatusCreated {
		return bodyBytes, nil
	}

	// TODO why is != 200 an error? there are valid codes in the 2xx, 3xx range
	// TODO we can remove doWithCustomErrorHandling, if we let the caller handle the response code?
	if response.StatusCode != http.StatusOK {
		err := f(response, bodyBytes, formatError(bodyBytes, response))
		if err != nil {
			if respLog.V(5).Enabled() {
				respLog = respLog.WithValues("body", string(bodyBytes))
			}
			respLog.V(1).Info("response is not StatusOK after custom error handling: " + err.Error())

			return bodyBytes, wrapResponseError(err, response.StatusCode)
		}
		return bodyBytes, nil
	}

	return bodyBytes, nil
}

func requestLogger(l logr.Logger, method string, uri string, body string) logr.Logger {
	log := l
	if log.V(5).Enabled() {
		log = log.WithValues(
			"method", method,
			"uri", uri,
		)
	}
	if log.V(5).Enabled() {
		log = log.WithValues("body", body)
	}
	return log
}

func responseLogger(l logr.Logger, response *http.Response, body string) logr.Logger {
	log := l.WithValues("status", response.StatusCode)
	if log.V(5).Enabled() {
		log = log.WithValues("header", response.Header)
		if response.TLS != nil {
			log = log.WithValues("TLSServerName", response.TLS.ServerName)
		}
		log = log.WithValues("body", body)
	}
	return log
}

func formatError(bodyBytes []byte, response *http.Response) error {
	t := "response body is empty"
	if len(bodyBytes) > 0 {
		var eResponse apierrors.ErrorResponse
		if err := json.Unmarshal(bodyBytes, &eResponse); err != nil {
			return errors.Wrapf(err, "cannot parse JSON response: '%s'", bodyBytes)
		}

		titles := make([]string, 0, len(eResponse.Errors))
		for _, e := range eResponse.Errors {
			titles = append(titles, e.Title)
		}
		t = strings.Join(titles, ", ")
	}

	return errors.Errorf("%s: %s", http.StatusText(response.StatusCode), t)
}

func (c *Client) AuthToken() (string, error) {
	data, err := c.get(api.Routes.Path("AuthToken"))
	if err != nil {
		return "", err
	}

	tr := &models.AuthTokenResponse{}
	err = json.Unmarshal(data, &tr)
	return tr.Token, err
}

func (c *Client) handleAuthorization(request *http.Request) error {
	if c.Settings.Token.AccessToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.Settings.Token.AccessToken)

		if oauth2Transport, ok := c.HttpClient.Transport.(*oauth2.Transport); ok {
			newToken, err := oauth2Transport.Source.Token()
			if err != nil {
				return errors.Wrap(err, "failed getting token")
			}
			if newToken.AccessToken != c.Settings.Token.AccessToken {
				log.Println("Refreshed expired token.")

				c.Settings.Token.AccessToken = newToken.AccessToken
				c.Settings.Token.RefreshToken = newToken.RefreshToken
				c.Settings.Token.Expiry = newToken.Expiry
				c.Settings.Token.TokenType = newToken.TokenType

				err := c.Settings.Save()
				if err != nil {
					return errors.Wrap(err, "failed saving refreshed token")
				}
			}
		}
	} else if c.Settings.User != "" && c.Settings.Password != "" {
		request.SetBasicAuth(c.Settings.User, c.Settings.Password)
	}
	return nil
}

func (c *Client) warnAboutVersionMismatch(serverVersion string) {
	if serverVersion == version.Version {
		return
	}
	c.DisableVersionWarning()

	ui := termui.NewUI()
	ui.Exclamation().Msg(
		fmt.Sprintf("Epinio server version (%s) doesn't match the client version (%s)", serverVersion, version.Version))
	ui.Exclamation().Msg("Update the client manually or run `epinio client-sync`")
}
