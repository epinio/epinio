package v1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Info endpoint", func() {
	It("includes the default builder image in the response", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/info",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var info models.InfoResponse
		err = json.Unmarshal(bodyBytes, &info)
		Expect(err).ToNot(HaveOccurred())

		Expect(info.DefaultBuilderImage).To(Equal("paketobuildpacks/builder:full"))
	})
})
