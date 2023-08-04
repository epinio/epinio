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

package v1_test

import (
	"net/http"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppValidateCV Endpoint", LApplication, func() {

	var (
		chartName string
		tempFile  string
		namespace string
		appName   string
		restart   bool
	)

	BeforeEach(func() {
		// Appchart
		chartName = catalog.NewTmpName("chart-")
		tempFile = env.MakeAppchart(chartName)

		// Namespace
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		// Application, references new chart
		appName = catalog.NewAppName()

		appCreateRequest := models.ApplicationCreateRequest{
			Name: appName,
			Configuration: models.ApplicationUpdateRequest{
				AppChart: chartName,
			},
		}
		bodyBytes, statusCode := appCreate(namespace, toJSON(appCreateRequest))
		Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))

		DeferCleanup(func() {
			env.DeleteNamespace(namespace)
			env.DeleteAppchart(tempFile)
		})
	})

	It("returns error when asking for a non existing app", func() {
		_, statusCode := appValidateCV(namespace, "noapp")
		Expect(statusCode).To(Equal(http.StatusNotFound))
	})

	It("returns error when validating for a non existing chart", func() {
		// create a new appchart that we can safely delete
		chartName2 := catalog.NewTmpName("chart-")
		tempFile2 := env.MakeAppchart(chartName2)

		appName2 := catalog.NewAppName()
		appCreateRequest := models.ApplicationCreateRequest{
			Name: appName2,
			Configuration: models.ApplicationUpdateRequest{
				AppChart: chartName2,
			},
		}
		bodyBytes, statusCode := appCreate(namespace, toJSON(appCreateRequest))
		Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))

		env.DeleteAppchart(tempFile2)

		_, statusCode = appValidateCV(namespace, appName2)
		Expect(statusCode).To(Equal(http.StatusNotFound))
	})

	It("returns ok when there are no chart values to validate", func() {
		bodyBytes, statusCode := appValidateCV(namespace, appName)
		ExpectResponseToBeOK(bodyBytes, statusCode)
	})

	It("returns ok for good chart values", func() {
		// unknowntype, badminton, maxbad - bad spec, no good values
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"fake":  "true",
				"foo":   "bar",
				"bar":   "sna",
				"floof": "3.1415926535",
				"fox":   "99",
				"cat":   "0.31415926535",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectResponseToBeOK(bodyBytes, statusCode)
	})

	It("fails for an unknown field", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"bogus": "x",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "bogus": Not known`)
	})

	It("fails for an unknown field type", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"unknowntype": "x",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "unknowntype": Bad spec: Unknown type "foofara"`)
	})

	It("fails for an integer field with a bad minimum", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"badminton": "0",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "badminton": Bad spec: Bad minimum "hello"`)
	})

	It("fails for an integer field with a bad maximum", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"maxbad": "0",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "maxbad": Bad spec: Bad maximum "world"`)
	})

	It("fails for a value out of range (< min)", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"floof": "-2",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "floof": Out of bounds, "-2" too small`)
	})

	It("fails for a value out of range (> max)", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"fox": "1000",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "fox": Out of bounds, "1000" too large`)
	})

	It("fails for a value out of range (not in enum)", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"bar": "fox",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "bar": Illegal string "fox"`)
	})

	It("fails for a non-integer value where integer required", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"fox": "hound",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "fox": Expected integer, got "hound"`)
	})

	It("fails for a non-numeric value where numeric required", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"cat": "dog",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "cat": Expected number, got "dog"`)
	})

	It("fails for a non-boolean value where boolean required", func() {
		request := models.ApplicationUpdateRequest{
			Restart: &restart,
			Settings: models.ChartValueSettings{
				"fake": "news",
			},
		}
		bodyBytes, statusCode := appUpdate(namespace, appName, toJSON(request))
		ExpectResponseToBeOK(bodyBytes, statusCode)

		bodyBytes, statusCode = appValidateCV(namespace, appName)
		ExpectBadRequestError(bodyBytes, statusCode, `Setting "fake": Expected boolean, got "news"`)
	})
})
