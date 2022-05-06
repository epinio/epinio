package v1_test

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	api "github.com/epinio/epinio/internal/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Users Namespace", func() {
	var request *http.Request
	var err error

	createNamespace := func(user, password, namespace string) {
		jsonRequest := fmt.Sprintf(`{"name":"%s"}`, namespace)
		endpoint := fmt.Sprintf("%s%s/namespaces", serverURL, api.Root)

		request, err = http.NewRequest(http.MethodPost, endpoint, strings.NewReader(jsonRequest))
		Expect(err).ToNot(HaveOccurred())
		request.SetBasicAuth(user, password)

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusCreated))
	}

	showNamespace := func(user, password, namespace string) *http.Response {
		endpoint := fmt.Sprintf("%s%s/namespaces/%s", serverURL, api.Root, namespace)
		request, err = http.NewRequest(http.MethodGet, endpoint, nil)
		Expect(err).ToNot(HaveOccurred())
		request.SetBasicAuth(user, password)

		response, err := env.Client().Do(request)
		Expect(err).ToNot(HaveOccurred())

		return response
	}

	Describe("having two user with 'user' role and an admin user", func() {
		var user1, passwordUser1 string
		var user2, passwordUser2 string
		var userAdmin, passwordAdmin string

		BeforeEach(func() {
			user1, passwordUser1 = env.CreateEpinioUser("user", nil)
			user2, passwordUser2 = env.CreateEpinioUser("user", nil)
			userAdmin, passwordAdmin = env.CreateEpinioUser("admin", nil)
		})

		AfterEach(func() {
			env.DeleteEpinioUser(user1)
			env.DeleteEpinioUser(user2)
			env.DeleteEpinioUser(userAdmin)
		})

		Describe("each user creates a namespace", func() {
			var namespaceUser1, namespaceUser2, namespaceAdmin string

			BeforeEach(func() {
				namespaceUser1 = catalog.NewNamespaceName()
				createNamespace(user1, passwordUser1, namespaceUser1)

				namespaceUser2 = catalog.NewNamespaceName()
				createNamespace(user2, passwordUser2, namespaceUser2)

				namespaceAdmin = catalog.NewNamespaceName()
				createNamespace(userAdmin, passwordAdmin, namespaceAdmin)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespaceUser1)
				env.DeleteNamespace(namespaceUser2)
				env.DeleteNamespace(namespaceAdmin)
			})

			When("user1 try to show a namespace", func() {
				It("can show his namespace", func() {
					response := showNamespace(user1, passwordUser1, namespaceUser1)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()
				})

				It("cannot show user2 namespace", func() {
					response := showNamespace(user1, passwordUser1, namespaceUser2)
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					response.Body.Close()
				})

				It("cannot show userAdmin namespace", func() {
					response := showNamespace(user1, passwordUser1, namespaceAdmin)
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					response.Body.Close()
				})
			})

			When("user2 try to show a namespace", func() {
				It("can show his namespace", func() {
					response := showNamespace(user2, passwordUser2, namespaceUser2)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()
				})

				It("cannot show user1 namespace", func() {
					response := showNamespace(user2, passwordUser2, namespaceUser1)
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					response.Body.Close()
				})

				It("cannot show userAdmin namespace", func() {
					response := showNamespace(user2, passwordUser2, namespaceAdmin)
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					response.Body.Close()
				})
			})

			When("userAdmin try to show a namespace", func() {
				It("can show every namespace", func() {
					response := showNamespace(userAdmin, passwordAdmin, namespaceUser1)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()

					response = showNamespace(userAdmin, passwordAdmin, namespaceUser2)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()

					response = showNamespace(userAdmin, passwordAdmin, namespaceAdmin)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()
				})
			})

			When("user1 delete its namespace and user2 recreate the same namespace", func() {
				var commonNamespace string
				BeforeEach(func() {
					commonNamespace = catalog.NewNamespaceName()
					createNamespace(user1, passwordUser1, commonNamespace)
					env.DeleteNamespace(commonNamespace)
					createNamespace(user2, passwordUser2, commonNamespace)

					fmt.Printf("User1 [%s] - User2 [%s] - namespace [%s]", user1, user2, commonNamespace)
				})

				It("can show his namespace", func() {
					response := showNamespace(user2, passwordUser2, commonNamespace)
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					response.Body.Close()
				})

				It("user1 cannot show user2 namespace", func() {
					response := showNamespace(user1, passwordUser1, commonNamespace)
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					response.Body.Close()
				})
			})
		})
	})
})
