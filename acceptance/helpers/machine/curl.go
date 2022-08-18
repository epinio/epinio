package machine

import (
	"crypto/tls"
	"io"
	"net/http"
)

// Curl is used to make requests against a server
func (m *Machine) Curl(method, uri string, requestBody io.Reader) (*http.Response, error) {
	request, err := http.NewRequest(method, uri, requestBody)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+m.token)
	return m.Client().Do(request)
}

func (m *Machine) Client() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // nolint:gosec // tests using self signed certs
			},
		},
	}
}
