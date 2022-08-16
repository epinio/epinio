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

var _ = Describe("ChartList Endpoint", func() {

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

		Expect(len(appcharts)).To(Equal(2))

		var names []string
		names = append(names, appcharts[0].Meta.Name)
		names = append(names, appcharts[1].Meta.Name)
		var desc []string
		desc = append(desc, appcharts[0].Description)
		desc = append(desc, appcharts[1].Description)
		var short []string
		short = append(short, appcharts[0].ShortDescription)
		short = append(short, appcharts[1].ShortDescription)
		var chart []string
		chart = append(chart, appcharts[0].HelmChart)
		chart = append(chart, appcharts[1].HelmChart)

		Expect(names).Should(ContainElements(
			"standard",
			"standard-stateful"))
		Expect(desc).Should(ContainElements(
			"Epinio standard support chart for application deployment",
			"Epinio standard support chart for stateful application deployment"))
		Expect(short).Should(ContainElements(
			"Epinio standard deployment",
			"Epinio standard stateful deployment"))
		Expect(chart).Should(ContainElements(
			"https://github.com/epinio/helm-charts/releases/download/epinio-application-0.1.21/epinio-application-0.1.21.tgz",
			"https://github.com/epinio/helm-charts/releases/download/epinio-application-stateful-0.1.21/epinio-application-stateful-0.1.21.tgz"))
	})
})
