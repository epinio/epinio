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

package machine

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// loopbackDevURL rewrites https://app.127.0.0.1.sslip.io:8443/... (and .nip.io) to dial
// 127.0.0.1 directly while preserving the original hostname for ingress routing (Host)
// and TLS SNI. This avoids flaky DNS lookups via systemd-resolved in CI.
func loopbackDevURL(raw string) (dialURL string, tlsServerName string, hostHeader string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return raw, "", "", false
	}
	host := u.Hostname()
	low := strings.ToLower(host)
	if !strings.HasSuffix(low, ".127.0.0.1.sslip.io") && !strings.HasSuffix(low, ".127.0.0.1.nip.io") {
		return raw, "", "", false
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	u2 := *u
	u2.Host = net.JoinHostPort("127.0.0.1", port)
	return u2.String(), host, host, true
}

// Curl is used to make requests against a server
func (m *Machine) Curl(method, uri string, requestBody io.Reader) (*http.Response, error) {
	dialURL, tlsServerName, hostHeader, rewrite := loopbackDevURL(uri)
	request, err := http.NewRequest(method, dialURL, requestBody)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(m.user, m.password)
	if rewrite {
		request.Host = hostHeader
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // nolint:gosec // tests using self signed certs
					ServerName:         tlsServerName,
				},
			},
		}
		return client.Do(request) // nolint:gosec // acceptance test helper, URI from test
	}
	return m.Client().Do(request) // nolint:gosec // acceptance test helper, URI from test
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
