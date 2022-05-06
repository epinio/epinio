// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth_test

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/auth/authfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Auth users", func() {
	var authService *auth.AuthService
	var fake = &authfakes.FakeSecretInterface{}

	rand.Seed(time.Now().UnixNano())

	BeforeEach(func() {
		authService = &auth.AuthService{
			SecretInterface: fake,
		}
	})

	Describe("GetUsers", func() {

		When("kubernetes returns an error", func() {
			It("returns an error and an empty slice", func() {
				fake.ListReturns(nil, errors.New("an error"))

				users, err := authService.GetUsers(context.Background())
				Expect(err).To(HaveOccurred())
				Expect(users).To(BeEmpty())
			})
		})

		When("kubernetes returns an empty list of secrets", func() {
			It("returns an empty slice", func() {
				fake.ListReturns(&corev1.SecretList{Items: []corev1.Secret{}}, nil)

				users, err := authService.GetUsers(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(users).To(BeEmpty())
			})
		})

		When("kubernetes returns some user secrets", func() {
			It("returns a list of users", func() {
				userSecrets := []corev1.Secret{
					newUserSecret("admin", "password", "admin", ""),
					newUserSecret("epinio", "mypass", "user", "workspace\nworkspace2"),
				}
				fake.ListReturns(&corev1.SecretList{Items: userSecrets}, nil)

				users, err := authService.GetUsers(context.Background())
				Expect(err).ToNot(HaveOccurred())
				Expect(users).To(HaveLen(2))
				Expect(users[0].Username).To(Equal("admin"))
				Expect(users[0].Namespaces).To(HaveLen(0))
				Expect(users[1].Namespaces).To(HaveLen(2))
			})
		})
	})

	Describe("GetUsersByAge", func() {

		When("kubernetes returns some user secrets", func() {
			It("returns a list of users ordered by CreationTime", func() {
				userSecrets := []corev1.Secret{
					newUserSecret("user1", "password", "admin", ""),
					newUserSecret("user2", "password", "user", "workspace\nworkspace2"),
					newUserSecret("user3", "password", "admin", ""),
					newUserSecret("user4", "password", "user", "workspace"),
					newUserSecret("user5", "password", "admin", ""),
					newUserSecret("user6", "password", "user", "workspace2"),
				}

				// shuffle secrets
				for i := range userSecrets {
					j := rand.Intn(i + 1)
					userSecrets[i], userSecrets[j] = userSecrets[j], userSecrets[i]
				}

				fake.ListReturns(&corev1.SecretList{Items: userSecrets}, nil)

				users, err := authService.GetUsersByAge(context.Background())
				Expect(err).ToNot(HaveOccurred())

				for i := 0; i < len(userSecrets)-1; i++ {
					Expect(users[i].CreatedAt).To(BeTemporally("<=", users[i+1].CreatedAt))
				}
			})
		})

		When("kubernetes returns some user secrets created at the same time", func() {
			now := metav1.NewTime(time.Now())

			It("returns a list of users ordered by Username", func() {
				userSecrets := []corev1.Secret{
					newUserSecret("user1", "password", "admin", ""),
					newUserSecret("user2", "password", "user", "workspace\nworkspace2"),
					newUserSecret("user3", "password", "admin", ""),
					newUserSecret("user4", "password", "user", "workspace"),
					newUserSecret("user5", "password", "admin", ""),
					newUserSecret("user6", "password", "user", "workspace2"),
				}

				// shuffle secrets
				for i := range userSecrets {
					userSecrets[i].CreationTimestamp = now

					j := rand.Intn(i + 1)
					userSecrets[i], userSecrets[j] = userSecrets[j], userSecrets[i]
				}

				fake.ListReturns(&corev1.SecretList{Items: userSecrets}, nil)

				users, err := authService.GetUsersByAge(context.Background())
				Expect(err).ToNot(HaveOccurred())

				for i := 0; i < len(userSecrets)-1; i++ {
					Expect(users[i].Username < users[i+1].Username).To(BeTrue())
				}
			})
		})
	})
})

func newUserSecret(username, password, role, namespaces string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				kubernetes.EpinioAPISecretRoleLabelKey: role,
			},
			CreationTimestamp: metav1.NewTime(newRandomDate()),
		},
		Data: map[string][]byte{
			"username":   []byte(username),
			"password":   []byte(password),
			"namespaces": []byte(namespaces),
		},
	}
}

func newRandomDate() time.Time {
	min := time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	max := time.Now().Unix()
	delta := max - min

	sec := rand.Int63n(delta) + min
	return time.Unix(sec, 0)
}
