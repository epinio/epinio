package v1_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppImportGit Endpoint", func() {
	var (
		appName   string
		namespace string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()

		By("creating application resource first")
		_, err := createApplication(appName, namespace, []string{})
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("POST /namespaces/:namespace/applications/:app/import-git", func() {
		It("imports the git repo in the blob store", func() {
			app := catalog.NewAppName()
			gitURL := "https://github.com/epinio/example-wordpress"
			data := url.Values{}
			data.Set("giturl", gitURL)
			data.Set("gitrev", "main")

			url := serverURL + v1.Root + "/" + v1.Routes.Path("AppImportGit", namespace, app)
			request, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
			Expect(err).ToNot(HaveOccurred())
			request.SetBasicAuth(env.EpinioUser, env.EpinioPassword)
			request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

			response, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred(), string(bodyBytes))
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var importResponse models.ImportGitResponse
			err = json.Unmarshal(bodyBytes, &importResponse)
			Expect(err).ToNot(HaveOccurred())
			Expect(importResponse.BlobUID).ToNot(BeEmpty())
			Expect(importResponse.BlobUID).To(MatchRegexp(".+-.+-.+-.+-.+"))
		})
	})
})
