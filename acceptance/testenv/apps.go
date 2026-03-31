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

package testenv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/gomega"
)

// ShowApp retrieves the application details directly from the API to allow
// acceptance tests to assert on the structured response.
func (m *EpinioEnv) ShowApp(appName, namespace string) models.App {
	settings, err := m.GetSettingsFrom(EpinioYAML())
	Expect(err).ToNot(HaveOccurred())

	apiEndpoint := strings.TrimRight(settings.API, "/")
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s", apiEndpoint, namespace, appName)

	response, err := m.Curl("GET", url, strings.NewReader(""))
	Expect(err).ToNot(HaveOccurred())
	defer func() {
		Expect(response.Body.Close()).To(Succeed())
	}()

	Expect(response.StatusCode).To(Equal(http.StatusOK))

	body, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred())

	var app models.App
	Expect(json.Unmarshal(body, &app)).To(Succeed())

	return app
}
