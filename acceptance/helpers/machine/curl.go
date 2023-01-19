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
	"net/http"
)

// Curl is used to make requests against a server
func (m *Machine) Curl(method, uri string, requestBody io.Reader) (*http.Response, error) {
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
				InsecureSkipVerify: true, // nolint:gosec // tests using self signed certs
			},
		},
	}
}
