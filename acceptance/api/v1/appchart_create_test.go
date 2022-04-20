package v1_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChartCreate Endpoint", func() {

	standardBall := "https://github.com/epinio/helm-charts/releases/download/epinio-application-0.1.15/epinio-application-0.1.15.tgz"

	When("creating a duplicate chart", func() {
		BeforeEach(func() {
			out, err := env.Epinio("", "apps", "chart", "create", "duplicate.api", "fox")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			out, err := env.Epinio("", "apps", "chart", "delete", "duplicate.api")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("fails to create the chart", func() {
			request := models.ChartCreateRequest{
				Name:      "duplicate.api",
				HelmChart: "placeholder",
			}
			b, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			url := serverURL + v1.Root + "/" + v1.Routes.Path("ChartCreate")
			response, err := env.Curl("POST", url, strings.NewReader(string(b)))

			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusConflict), string(bodyBytes))
			Expect(string(bodyBytes)).To(Equal(`{"errors":[{"status":409,"title":"Application Chart 'duplicate.api' already exists","details":""}]}`))
		})
	})

	When("creating a new app chart", func() {
		AfterEach(func() {
			out, err := env.Epinio("", "apps", "chart", "delete", "standard.direct")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("creates the app chart resource", func() {
			request := models.ChartCreateRequest{
				Name:        "standard.direct",
				Description: "Direct standard",
				ShortDesc:   "Direct url to tarball standard",
				HelmChart:   standardBall,
			}
			b, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			url := serverURL + v1.Root + "/" + v1.Routes.Path("ChartCreate")
			response, err := env.Curl("POST", url, strings.NewReader(string(b)))

			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()

			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))

			out, err := proc.Kubectl("get", "appcharts", "-n", "epinio",
				"standard.direct", "-o", "json")
			Expect(err).ToNot(HaveOccurred(), out)

			var result map[string]interface{}
			json.Unmarshal([]byte(out), &result)
			spec := result["spec"].(map[string]interface{})

			Expect(spec["description"].(string)).To(Equal("Direct standard"))
			Expect(spec["shortDescription"].(string)).To(Equal("Direct url to tarball standard"))
			Expect(spec["helmChart"].(string)).To(Equal(standardBall))
			Expect(spec["helmRepo"]).To(BeNil())
		})
	})
})
