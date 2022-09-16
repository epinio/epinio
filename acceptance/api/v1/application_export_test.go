package v1_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppPart Endpoint", func() {
	var (
		namespace string
		app       string
		domain    string
	)
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		domain = catalog.NewTmpName("exportdomain-") + ".org"

		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		app = catalog.NewAppName()
		env.MakeRoutedContainerImageApp(app, 1, containerImageURL, domain)
	})

	AfterEach(func() {
		env.DeleteApp(app)
		env.DeleteNamespace(namespace)
	})

	It("retrieves the named application part", func() {
		// The testsuite checks using only part `values`, as the smallest possible, and also text.
		// The parts `chart` (and, in the future, maybe, `image`) are much larger, and binary.

		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/%s/part/values",
			serverURL, v1.Root, namespace, app), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		Expect(string(bodyBytes)).To(Equal(fmt.Sprintf(`epinio:
  appName: %s
  configurations: []
  env: []
  imageURL: splatform/sample-app
  ingress: null
  replicaCount: 1
  routes:
  - domain: %s
    id: %s
    path: /
  stageID: ""
  start: null
  tlsIssuer: epinio-ca
  username: admin@epinio.io
`, app, domain, domain)))
	})

	It("returns a 404 when the namespace does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/applications/%s/part/values",
			serverURL, v1.Root, app), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 404 when the app does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/bogus/part/values",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 400 when the part does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/%s/part/bogus",
			serverURL, v1.Root, namespace, app), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
	})
})
