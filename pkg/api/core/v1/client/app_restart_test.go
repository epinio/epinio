package client_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func DescribeAppRestart() {

	var epinioClient *client.Client
	var statusCode int
	var responseBody string

	JustBeforeEach(func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			fmt.Fprint(w, responseBody)
		}))

		epinioClient = client.New(context.Background(), &settings.Settings{API: srv.URL})
	})

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
}
