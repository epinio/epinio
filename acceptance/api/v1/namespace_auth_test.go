package v1_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Users Namespace", func() {
	var request *http.Request
	var err error

	createNamespace := func(user, namespace string) {
		jsonRequest := fmt.Sprintf(`{"name":"%s"}`, namespace)
		endpoint := fmt.Sprintf("%s%s/namespaces", serverURL, api.Root)

		request, err = http.NewRequest(http.MethodPost, endpoint, strings.NewReader(jsonRequest))
		Expect(err).ToNot(HaveOccurred())
		request.Header.Set("Authorization", "Bearer "+env.GetUserToken(user))

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusCreated))
	}

	namespaceRequest := func(user, endpoint string) *http.Response {
		request, err = http.NewRequest(http.MethodGet, endpoint, nil)
		Expect(err).ToNot(HaveOccurred())
		request.Header.Set("Authorization", "Bearer "+env.GetUserToken(user))

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())

		return response
	}

	showNamespace := func(user, namespace string) *http.Response {
		endpoint := fmt.Sprintf("%s%s/namespaces/%s", serverURL, api.Root, namespace)
		return namespaceRequest(user, endpoint)
	}

	listNamespaces := func(user string) *http.Response {
		endpoint := fmt.Sprintf("%s%s/namespaces", serverURL, api.Root)
		return namespaceRequest(user, endpoint)
	}

	Describe("having two user with 'user' role and an admin user", func() {

		Describe("each user creates a namespace", func() {
			var namespaceUser1, namespaceUser2, namespaceAdmin string

			BeforeEach(func() {
				namespaceUser1 = catalog.NewNamespaceName()
				createNamespace("user1@epinio.io", namespaceUser1)

				namespaceUser2 = catalog.NewNamespaceName()
				createNamespace("user2@epinio.io", namespaceUser2)

				namespaceAdmin = catalog.NewNamespaceName()
				createNamespace("admin@epinio.io", namespaceAdmin)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespaceUser1)
				env.DeleteNamespace(namespaceUser2)
				env.DeleteNamespace(namespaceAdmin)
			})

			When("user1 tries to show a namespace", func() {
				It("shows the user's namespace", func() {
					response := showNamespace("user1@epinio.io", namespaceUser1)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()
				})

				It("doesn't show the other user's namespace", func() {
					response := showNamespace("user1@epinio.io", namespaceUser2)
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					response.Body.Close()
				})

				It("doesn't show the admin's namespace", func() {
					response := showNamespace("user1@epinio.io", namespaceAdmin)
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					response.Body.Close()
				})
			})

			When("user1 tries to list all the namespaces", func() {
				It("list only the user1 namespace", func() {
					response := listNamespaces("user1@epinio.io")
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					defer response.Body.Close()

					var namespaceList models.NamespaceList
					err := json.NewDecoder(response.Body).Decode(&namespaceList)
					Expect(err).ToNot(HaveOccurred())

					Expect(namespaceList).To(HaveLen(1))
					Expect(namespaceList[0].Meta.Name).To(Equal(namespaceUser1))
				})
			})

			When("an admin user tries to show a namespace", func() {
				It("shows every namespace", func() {
					response := showNamespace("admin@epinio.io", namespaceUser1)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()

					response = showNamespace("admin@epinio.io", namespaceUser2)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()

					response = showNamespace("admin@epinio.io", namespaceAdmin)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()
				})
			})

			When("an admin user tries to list all the namespaces", func() {
				It("list every namespace", func() {
					response := listNamespaces("admin@epinio.io")
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					defer response.Body.Close()

					var namespaceList models.NamespaceList
					err := json.NewDecoder(response.Body).Decode(&namespaceList)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(namespaceList)).To(BeNumerically(">=", 3))

					// check that within the namespaces returned there are the one that we created
					namespaceListMap := make(map[string]struct{})
					for _, namespace := range namespaceList {
						namespaceListMap[namespace.Meta.Name] = struct{}{}
					}

					Expect(namespaceListMap).To(HaveKey(namespaceUser1))
					Expect(namespaceListMap).To(HaveKey(namespaceUser2))
					Expect(namespaceListMap).To(HaveKey(namespaceAdmin))
				})
			})

			When("a user deletes a namespace and another user recreates the same namespace", func() {
				var commonNamespace string

				BeforeEach(func() {
					commonNamespace = catalog.NewNamespaceName()
					createNamespace("user1@epinio.io", commonNamespace)
					env.DeleteNamespace(commonNamespace)
					createNamespace("user2@epinio.io", commonNamespace)
				})

				It("shows the user's namespace", func() {
					response := showNamespace("user2@epinio.io", commonNamespace)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()
				})

				It("doesn't show the other user's namespace", func() {
					response := showNamespace("user1@epinio.io", commonNamespace)
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					response.Body.Close()
				})
			})
		})
	})
})
