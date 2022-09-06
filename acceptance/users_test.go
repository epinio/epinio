package acceptance_test

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Users", func() {
	var request *http.Request
	var err error
	var uri string

	BeforeEach(func() {
		uri = fmt.Sprintf("%s%s/info", serverURL, v1.Root)
		request, err = http.NewRequest("GET", uri, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("an existing user", func() {

		Specify("can authenticate with token", func() {
			request.Header.Set("Authorization", "Bearer "+env.GetUserToken("user1@epinio.io"))
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		Specify("cannot authenticate no credentials", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})

	When("an existing user is deleted", func() {
		var user, password string

		BeforeEach(func() {
			user, password = env.CreateEpinioUser("user", nil)
			request.SetBasicAuth(user, password)
		})

		Specify("the user can no longer authenticate with basic auth", func() {
			env.DeleteEpinioUser(user)
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})

	When("user doesn't exist", func() {
		Specify("the response should be 401", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})

	Describe("a regular user", func() {

		Specify("can describe its namespace", func() {
			updateToken("user1@epinio.io")
			namespace := catalog.NewNamespaceName()
			env.SetupNamespace(namespace)

			out, err := env.Epinio("", "namespace", "show", namespace)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Name", namespace),
					WithRow("Created", WithDate()),
					WithRow("Applications"),
					WithRow("Configurations"),
				),
			)

			env.DeleteNamespace(namespace)
		})

		Specify("cannot describe another namespace", func() {
			// create user2 namespace
			updateToken("user2@epinio.io")
			namespace := catalog.NewNamespaceName()
			env.SetupNamespace(namespace)

			updateToken("user1@epinio.io")
			out, err := env.Epinio("", "namespace", "show", namespace)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Forbidden: user unauthorized"))

			// cleanup
			updateToken("user2@epinio.io")
			env.DeleteNamespace(namespace)
		})
	})

	Describe("an admin user", func() {

		Specify("can describe any namespace", func() {
			updateToken("admin@epinio.io")

			out, err := env.Epinio("", "namespace", "show", "workspace")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Forbidden: user unauthorized"))
		})
	})
})
