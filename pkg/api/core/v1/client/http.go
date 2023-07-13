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
	"os"
	"path/filepath"

	"github.com/epinio/epinio/helpers/termui"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/version"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"golang.org/x/oauth2"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// RequestHandler is a method that will return a *http.Request from a method and url
type RequestHandler func(method, url string) (*http.Request, error)

// ResponseHandler is a method that will handle a *http.Response to return a typed struct
type ResponseHandler[T any] func(httpResponse *http.Response) (T, error)

type APIError struct {
	StatusCode int
	Err        *apierrors.ErrorResponse
}

func (e *APIError) Error() string {
	if e.Err != nil && len(e.Err.Errors) > 0 {
		return e.Err.Errors[0].Error()
	}
	return "empty"
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

func Get[T any](c *Client, endpoint string, response T) (T, error) {
	return Do(c, endpoint, http.MethodGet, nil, response)
}

func Post[T any](c *Client, endpoint string, request any, response T) (T, error) {
	return Do(c, endpoint, http.MethodPost, request, response)
}

func Patch[T any](c *Client, endpoint string, request any, response T) (T, error) {
	return Do(c, endpoint, http.MethodPatch, request, response)
}

func Delete[T any](c *Client, endpoint string, request any, response T) (T, error) {
	return Do(c, endpoint, http.MethodDelete, request, response)
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

	for key, value := range c.customHeaders {
		request.Header.Set(key, value)
	}

	response, err := c.HttpClient.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to POST to upload")
	}

	if response.StatusCode >= http.StatusBadRequest {
		return nil, handleError(c.log, response)
	}

	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading response")
	}
	return bodyBytes, nil
}

// Do will execute a common JSON http request, marshalling the provided body, and unmarshalling the httpResponse into the response struct
func Do[T any](c *Client, endpoint string, method string, requestBody any, response T) (T, error) {
	jsonRequestHandler := NewJSONRequestHandler(requestBody)
	jsonResponseHandler := NewJSONResponseHandler[T](c.log, response)
	return DoWithHandlers(c, endpoint, method, jsonRequestHandler, jsonResponseHandler)
}

// Do will execute a plain http request, returning the *http.Response
func (c *Client) Do(endpoint string, method string, body io.Reader) (*http.Response, error) {
	requestHandler := NewHTTPRequestHandler(body)
	responseHandler := NewHTTPResponseHandler()
	return DoWithHandlers(c, endpoint, method, requestHandler, responseHandler)
}

// DoWithHandlers will execute the request using the provided RequestHandler and ResponseHandler.
func DoWithHandlers[T any](
	c *Client,
	endpoint string,
	method string,
	requestHandler RequestHandler,
	responseHandler ResponseHandler[T],
) (T, error) {
	var response T

	if requestHandler == nil {
		return response, errors.New("missing request handler")
	}
	if responseHandler == nil {
		return response, errors.New("missing response handler")
	}

	url := fmt.Sprintf("%s%s/%s", c.Settings.API, api.Root, endpoint)
	request, err := requestHandler(method, url)
	if err != nil {
		return response, errors.Wrap(err, "building request")
	}

	err = c.handleAuthorization(request)
	if err != nil {
		return response, err
	}

	for key, value := range c.customHeaders {
		request.Header.Set(key, value)
	}

	reqLog := requestLogger(c.log, request)
	reqLog.V(1).Info("executing request")

	httpResponse, err := c.HttpClient.Do(request)
	if err != nil {
		return response, errors.Wrap(err, "making the request")
	}

	serverVersion := httpResponse.Header.Get(api.VersionHeader)
	if c.VersionWarningEnabled() && serverVersion != "" {
		c.warnAboutVersionMismatch(serverVersion)
	}

	// the server returned an error
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return response, handleError(c.log, httpResponse)
	}

	return responseHandler(httpResponse)
}

// NewHTTPRequestHandler return a plain *http.Request with the provided body
func NewHTTPRequestHandler(body io.Reader) RequestHandler {
	return func(method, url string) (*http.Request, error) {
		return http.NewRequest(method, url, body)
	}
}

// NewJSONRequestHandler creates a request marshalling the provided body into JSON
func NewJSONRequestHandler(body any) RequestHandler {
	return func(method, url string) (*http.Request, error) {
		var reader io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, errors.Wrap(err, "encoding JSON requestBody")
			}
			reader = bytes.NewReader(b)
		}

		request, err := http.NewRequest(method, url, reader)
		if err != nil {
			return nil, errors.Wrap(err, "building request")
		}
		return request, nil
	}
}

// NewFileUploadRequestHandler creates a multipart/form-data request to upload the provided file
func NewFileUploadRequestHandler(file *os.File) RequestHandler {
	return func(method, url string) (*http.Request, error) {
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
		request, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build request")
		}

		request.Header.Add("Content-Type", writer.FormDataContentType())

		return request, nil
	}
}

// NewHTTPResponseHandler is a no-op ResponseHandler that returns the plain *http.Response that can be directly used
func NewHTTPResponseHandler[T *http.Response]() ResponseHandler[T] {
	return func(httpResponse *http.Response) (T, error) {
		return httpResponse, nil
	}
}

// NewJSONResponseHandler will try to unmarshal the response body into the provided struct
func NewJSONResponseHandler[T any](logger logr.Logger, response T) ResponseHandler[T] {
	return func(httpResponse *http.Response) (T, error) {
		defer httpResponse.Body.Close()

		bodyBytes, err := io.ReadAll(httpResponse.Body)
		respLog := responseLogger(logger, httpResponse, string(bodyBytes))
		if err != nil {
			respLog.V(1).Error(err, "failed to read response body")
			return response, errors.Wrap(err, "reading response body")
		}

		respLog.V(1).Info("response received", "status", httpResponse.StatusCode)

		if err := json.Unmarshal(bodyBytes, &response); err != nil {
			return response, errors.Wrap(err, "decoding JSON response")
		}

		logger.V(1).Info("response decoded", "response", response)

		return response, nil
	}
}

func handleError(logger logr.Logger, response *http.Response) error {
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)

	if logger.V(5).Enabled() {
		logger = logger.WithValues("body", string(bodyBytes))
	}

	if err != nil {
		logger.Error(err, "failed to read response body")
		return errors.Wrap(err, "reading response body")
	}

	var responseErr apierrors.ErrorResponse
	err = json.Unmarshal(bodyBytes, &responseErr)
	if err != nil {
		logger.Error(err, "decoding json error")
		return errors.Wrap(err, "parsing error response")
	}

	epinioError := &APIError{
		StatusCode: response.StatusCode,
		Err:        &responseErr,
	}

	logger.V(1).Info("response is not StatusOK: " + epinioError.Error())

	return epinioError
}

func requestLogger(log logr.Logger, request *http.Request) logr.Logger {
	var bodyString string

	if request.Body != nil {
		if body, err := request.GetBody(); err == nil {
			if bodyBytes, err := io.ReadAll(body); err == nil {
				bodyString = string(bodyBytes)
			}
		}
	}

	if log.V(5).Enabled() {
		log = log.WithValues(
			"method", request.Method,
			"uri", request.RequestURI,
			"body", bodyString,
			"header", request.Header,
		)
	}

	return log
}

func responseLogger(log logr.Logger, response *http.Response, body string) logr.Logger {
	log = log.WithValues("status", response.StatusCode)

	if log.V(5).Enabled() {
		log = log.WithValues(
			"body", body,
			"header", response.Header,
		)
		if response.TLS != nil {
			log = log.WithValues("TLSServerName", response.TLS.ServerName)
		}
	}

	return log
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
