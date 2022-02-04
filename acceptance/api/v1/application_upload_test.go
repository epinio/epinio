package v1_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppUpload Endpoint", func() {
	var (
		namespace string
		url       string
		path      string
		request   *http.Request
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})
	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	JustBeforeEach(func() {
		url = serverURL + v1.Root + "/" + v1.Routes.Path("AppUpload", namespace, "testapp")
		var err error
		request, err = uploadRequest(url, path)
		Expect(err).ToNot(HaveOccurred())
	})

	When("uploading a new dir", func() {
		BeforeEach(func() {
			path = testenv.TestAssetPath("sample-app.tar")
		})

		It("returns the app response", func() {
			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp).ToNot(BeNil())
			defer resp.Body.Close()

			bodyBytes, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			r := &models.UploadResponse{}
			err = json.Unmarshal(bodyBytes, &r)
			Expect(err).ToNot(HaveOccurred())

			Expect(r.BlobUID).ToNot(BeEmpty())
		})
	})
})
