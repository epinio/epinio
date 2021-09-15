package epinioapi

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

	"github.com/pkg/errors"
)

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
		return nil, fmt.Errorf("server status code: %s\n%s", http.StatusText(response.StatusCode), string(bodyBytes))
	}

	// object was not created, but status was ok?
	return bodyBytes, nil
}

func (c *Client) do(endpoint, method, requestBody string) ([]byte, error) {
	uri := fmt.Sprintf("%s/%s", c.URL, endpoint)
	c.log.Info(fmt.Sprintf("%s %s", method, uri))
	c.log.V(1).Info(requestBody)
	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		return []byte{}, err
	}

	request.SetBasicAuth(c.user, c.password)
	response, err := (&http.Client{}).Do(request)
	if err != nil {
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

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return []byte{}, errors.Wrap(err, "reading response body")
	}

	if response.StatusCode == http.StatusCreated {
		return bodyBytes, nil
	}

	if response.StatusCode != http.StatusOK {
		return []byte{}, formatError(bodyBytes, response)
	}

	return bodyBytes, nil
}

type errorFunc = func(response *http.Response, bodyBytes []byte, err error) error

func (c *Client) doWithCustomErrorHandling(endpoint, method, requestBody string, f errorFunc) ([]byte, error) {

	uri := fmt.Sprintf("%s/%s", c.URL, endpoint)
	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		return []byte{}, err
	}

	request.SetBasicAuth(c.user, c.password)

	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return []byte{}, err
	}
	defer response.Body.Close()

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return []byte{}, err
	}

	if response.StatusCode == http.StatusCreated {
		return bodyBytes, nil
	}

	if response.StatusCode != http.StatusOK {
		return []byte{}, f(response, bodyBytes, formatError(bodyBytes, response))
	}

	return bodyBytes, nil
}

func formatError(bodyBytes []byte, response *http.Response) error {
	var eResponse api.ErrorResponse
	if err := json.Unmarshal(bodyBytes, &eResponse); err != nil {
		return errors.Wrapf(err, "cannot parse JSON response: '%s'", bodyBytes)
	}

	titles := make([]string, 0, len(eResponse.Errors))
	for _, e := range eResponse.Errors {
		titles = append(titles, e.Title)
	}
	t := strings.Join(titles, ", ")

	if response.StatusCode == http.StatusInternalServerError {
		return errors.Errorf("%s: %s", http.StatusText(response.StatusCode), t)
	}
	return errors.New(t)
}
