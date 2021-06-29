package machine

import (
	"crypto/tls"
	"net/http"
	"strings"
)

// Curl is used to make requests against a server
func (m *Machine) Curl(method, uri string, requestBody *strings.Reader) (*http.Response, error) {
	request, err := http.NewRequest(method, uri, requestBody)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(m.user, m.password)
	return m.Client().Do(request)
}

func (m *Machine) Client() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // self signed certs
			},
		},
	}
}
