package v1_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChartMatch Endpoints", LAppchart, func() {

	It("lists the app chart names matching the prefix, none", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/appchartsmatch/fox",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		Expect(err).ToNot(HaveOccurred())
		Expect(string(bodyBytes)).To(Equal(`{}`))
	})

	It("lists the app chart names matching the prefix, standard", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/appchartsmatch/stan",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		Expect(err).ToNot(HaveOccurred())
		Expect(string(bodyBytes)).To(MatchRegexp(`{"names":\[.*\]`))
		Expect(string(bodyBytes)).To(MatchRegexp(`"standard"`))
	})

	It("lists the app chart names matching the prefix, all", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/appchartsmatch",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		Expect(err).ToNot(HaveOccurred())
		Expect(string(bodyBytes)).To(MatchRegexp(`{"names":\[.*\]`))
		Expect(string(bodyBytes)).To(MatchRegexp(`"standard"`))
	})
})
