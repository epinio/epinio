package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/internal/services"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServicePlans API Application Endpoints", func() {
	Describe("GET /api/v1/serviceclasses/:serviceclass/serviceplans", func() {
		var servicePlanNames []string
		var servicePlanDescs []string
		var servicePlanFrees []bool

		It("lists all serviceplans", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s/api/v1/serviceclasses/%s/serviceplans",
				serverURL, "mariadb"), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var data services.ServicePlanList
			err = json.Unmarshal(bodyBytes, &data)
			Expect(err).ToNot(HaveOccurred())
			for _, servicePlan := range data {
				servicePlanNames = append(servicePlanNames, servicePlan.Name)
				servicePlanDescs = append(servicePlanDescs, servicePlan.Description)
				servicePlanFrees = append(servicePlanFrees, servicePlan.Free)
			}
			Expect(servicePlanNames).Should(ContainElements("10-1-26", "10-1-28", "10-3-20", "10-3-16", "10-3-17"))
			Expect(servicePlanDescs).Should(ContainElements("Fast, reliable, scalable, and easy to use open-source relational database system. MariaDB Server is intended for mission-critical, heavy-load production systems as well as for embedding into mass-deployed software. Highly available MariaDB cluster."))
			Expect(servicePlanFrees).Should(ContainElements(true, true))
		})
	})
})
