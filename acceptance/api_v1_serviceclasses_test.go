package acceptance_test

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

var _ = Describe("ServiceClasses API Application Endpoints", func() {
	Describe("GET /api/v1/serviceclasses", func() {
		var serviceClassNames []string
		var serviceClassDescs []string
		var serviceClassBroker []string

		It("lists all serviceclasses", func() {
			response, err := Curl("GET", fmt.Sprintf("%s/api/v1/serviceclasses",
				serverURL), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var data services.ServiceClassList
			err = json.Unmarshal(bodyBytes, &data)
			Expect(err).ToNot(HaveOccurred())
			for _, serviceClass := range data {
				serviceClassNames = append(serviceClassNames, serviceClass.Name)
				serviceClassDescs = append(serviceClassDescs, serviceClass.Description)
				serviceClassBroker = append(serviceClassBroker, serviceClass.Broker)
			}
			Expect(serviceClassNames).Should(ContainElements("mariadb", "mongodb", "mysql", "postgresql", "rabbitmq"))
			Expect(serviceClassDescs).Should(ContainElements("Helm Chart for rabbitmq"))
			Expect(serviceClassBroker).Should(ContainElements("minibroker"))
			Expect(serviceClassBroker).ShouldNot(ContainElements("google"))
		})
	})
})
