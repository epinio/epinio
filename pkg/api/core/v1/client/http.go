package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	api "github.com/epinio/epinio/internal/api/v1"
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
	uri := fmt.Sprintf("%s/%s", c.URL, endpoint)

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

	request.SetBasicAuth(c.user, c.password)
	request.Header.Add("Content-Type", writer.FormDataContentType())

	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to POST to upload")
	}
	defer response.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(response.Body)
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
	uri := fmt.Sprintf("%s/%s", c.URL, endpoint)
	c.log.Info(fmt.Sprintf("%s %s", method, uri))

	reqLog := requestLogger(c.log, method, uri, requestBody)

	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		reqLog.V(1).Error(err, "cannot build request")
		return []byte{}, err
	}

	request.SetBasicAuth(c.user, c.password)

	response, err := (&http.Client{}).Do(request)
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

	respLog := responseLogger(c.log, response)

	bodyBytes, err := ioutil.ReadAll(response.Body)
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
		respLog.V(1).Error(err, "response is not StatusOK")
		return bodyBytes, wrapResponseError(err, response.StatusCode)
	}

	return bodyBytes, nil
}

type errorFunc = func(response *http.Response, bodyBytes []byte, err error) error

// doWithCustomErrorHandling has a special handler for "response type" errors.
// These are errors where the server send a valid http response, but the status
// code is not 200.
// The errorFunc allows us to inspect the response, even unmarshal it into an
// api.ErrorResponse and change the returned error.
// Note: it's only used by ServiceDelete and that could be changed to transmit
// it's data in a normal Response, instead of an error?
func (c *Client) doWithCustomErrorHandling(endpoint, method, requestBody string, f errorFunc) ([]byte, error) {

	uri := fmt.Sprintf("%s/%s", c.URL, endpoint)
	c.log.Info(fmt.Sprintf("%s %s", method, uri))

	reqLog := requestLogger(c.log, method, uri, requestBody)

	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		reqLog.V(1).Error(err, "cannot build request")
		return []byte{}, err
	}

	request.SetBasicAuth(c.user, c.password)

	response, err := (&http.Client{}).Do(request)
	if err != nil {
		reqLog.V(1).Error(err, "request failed")
		return []byte{}, err
	}
	defer response.Body.Close()
	reqLog.V(1).Info("request finished")

	respLog := responseLogger(c.log, response)

	bodyBytes, err := ioutil.ReadAll(response.Body)
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
			respLog.V(1).Error(err, "response is not StatusOK after custom error handling")
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
	if log.V(15).Enabled() {
		log = log.WithValues("body", body)
	}
	return log
}

func responseLogger(l logr.Logger, response *http.Response) logr.Logger {
	log := l.WithValues("status", response.StatusCode)
	if log.V(15).Enabled() {
		log = log.WithValues("header", response.Header)
		if response.TLS != nil {
			log = log.WithValues("TLSServerName", response.TLS.ServerName)
		}
	}
	return log
}

func formatError(bodyBytes []byte, response *http.Response) error {
	t := "response body is empty"
	if len(bodyBytes) > 0 {
		var eResponse api.ErrorResponse
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
