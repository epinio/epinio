package client_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client Apps unit tests", func() {
	var epinioClient *client.Client
	var statusCode int
	var responseBody string

	BeforeEach(func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			fmt.Fprint(w, responseBody)
		}))

		epinioClient = client.New(tracelog.NewLogger(), srv.URL, "", "", "")
	})

	Describe("AppRestart", func() {
		When("app restart successfully", func() {
			BeforeEach(func() {
				statusCode = 200
				responseBody = `{ "status": "OK" }`
			})

			It("returns no error", func() {
				err := epinioClient.AppRestart("namespace-foo", "appname")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("something bad happened", func() {
			BeforeEach(func() {
				statusCode = 500
				responseBody = `{
					"errors": [
						{
							"status": 500,
							"title": "Error title",
							"details": "something bad happened"
						}
					]
				}`
			})

			It("it returns an error", func() {
				err := epinioClient.AppRestart("namespace-foo", "appname")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
