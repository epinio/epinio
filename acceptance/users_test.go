package acceptance_test

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
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
		var user, password string

		BeforeEach(func() {
			user, password = env.CreateEpinioUser("user", nil)
		})
		AfterEach(func() {
			env.DeleteEpinioUser(user)
		})

		Specify("can authenticate with basic auth", func() {
			request.SetBasicAuth(user, password)
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		Specify("cannot authenticate no credentials or cookie", func() {
			// First request with basicauth to get the cookie
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

		AfterEach(func() {
			// Ensure it's deleted even if test fails
			env.DeleteEpinioUser(user)
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
		var user, password string
		var namespace string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupNamespace(namespace)
			user, password = env.CreateEpinioUser("user", []string{"workspace", "workspace2"})
		})

		AfterEach(func() {
			env.DeleteEpinioUser(user)
			env.DeleteNamespace(namespace)
		})

		Specify("can describe its namespace", func() {
			uri := fmt.Sprintf("%s%s/namespaces/workspace", serverURL, v1.Root)
			request, err := http.NewRequest("GET", uri, nil)
			Expect(err).ToNot(HaveOccurred())

			request.SetBasicAuth(user, password)
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		Specify("cannot describe another namespace", func() {
			uri := fmt.Sprintf("%s%s/namespaces/%s", serverURL, v1.Root, namespace)
			request, err := http.NewRequest("GET", uri, nil)
			Expect(err).ToNot(HaveOccurred())

			request.SetBasicAuth(user, password)
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
		})
	})

	Describe("an admin user", func() {
		var user, password string

		BeforeEach(func() {
			user, password = env.CreateEpinioUser("admin", nil)
		})

		AfterEach(func() {
			env.DeleteEpinioUser(user)
		})

		Specify("can describe any namespace", func() {
			uri := fmt.Sprintf("%s%s/namespaces/workspace", serverURL, v1.Root)
			request, err := http.NewRequest("GET", uri, nil)
			Expect(err).ToNot(HaveOccurred())

			request.SetBasicAuth(user, password)
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
