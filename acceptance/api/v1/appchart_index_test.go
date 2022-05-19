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

		Expect(len(appcharts)).To(Equal(1))
		Expect(appcharts[0].Meta.Name).To(Equal("standard"))
		Expect(appcharts[0].Description).To(Equal("Epinio standard support chart for application deployment"))
		Expect(appcharts[0].ShortDescription).To(Equal("Epinio standard deployment"))
		Expect(appcharts[0].HelmChart).To(MatchRegexp("https://github\\.com/epinio/helm-charts/releases/download/epinio-application-.*/epinio-application-.*\\.tgz"))
	})
})
