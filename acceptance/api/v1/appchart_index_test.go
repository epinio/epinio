package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChartLists Endpoint", func() {

	It("lists the known app charts", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/appcharts",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var appcharts models.AppChartList
		err = json.Unmarshal(bodyBytes, &appcharts)
		Expect(err).ToNot(HaveOccurred())

		Expect(appcharts).To(ContainElements(models.AppChartList{
			models.AppChart{
				Name:             "standard",
				Description:      "Epinio standard support chart for application deployment",
				ShortDescription: "Epinio standard deployment",
				HelmChart:        "epinio-application:0.1.15",
				HelmRepo:         "https://epinio.github.io/helm-charts",
			},
		}))
	})
})
