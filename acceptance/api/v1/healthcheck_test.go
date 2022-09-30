package v1_test

import (
	"fmt"
	"io"
	"net/http"

	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Healthcheck endpoint", func() {
	It("returns OK (HTTP 200) without authentication", func() {
		request, err := http.NewRequest("GET", fmt.Sprintf("%s/ready", serverURL), nil)
		Expect(err).ToNot(HaveOccurred())

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
	})

	It("doesn't include the epinio server version in a header (non-authenticated request)", func() {
		request, err := http.NewRequest("GET", fmt.Sprintf("%s/ready", serverURL), nil)
		Expect(err).ToNot(HaveOccurred())

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		versionHeader := response.Header.Get(v1.VersionHeader)
		Expect(versionHeader).To(BeEmpty())
	})
})
