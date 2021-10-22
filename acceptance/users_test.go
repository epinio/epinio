package acceptance_test

import (
	"fmt"
	"net/http"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Users", func() {
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
			user, password = env.CreateEpinioUser()
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

		Specify("can authenticate with a session cookie", func() {
			// First request with basicauth to get the cookie
			request.SetBasicAuth(user, password)
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())

			// New request without basic auth credentials and just a session cookie
			Expect(len(resp.Header["Set-Cookie"])).To(Equal(1))
			cookie := resp.Header["Set-Cookie"][0]
			request, err = http.NewRequest("GET", uri, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			request.Header.Set("Cookie", cookie)
			resp, err = env.Client().Do(request)
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
		var cookie, user, password string

		BeforeEach(func() {
			user, password = env.CreateEpinioUser()

			// First request with basicauth to get the cookie
			request.SetBasicAuth(user, password)
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(resp.Header["Set-Cookie"])).To(Equal(1))
			// New request without basic auth credentials and just a session cookie
			cookie = resp.Header["Set-Cookie"][0]
		})

		Specify("the user can no longer authenticate with the session cookie", func() {
			request, err = http.NewRequest("GET", uri, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			request.Header.Set("Cookie", cookie)
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			env.DeleteEpinioUser(user)
			resp, err = env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
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
})
