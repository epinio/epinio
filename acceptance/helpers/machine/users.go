package machine

import (
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/internal/names"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateEpinioUser creates a new "user" BasicAuth Secret labeled as an Epinio User.
func (m *Machine) CreateEpinioUser(role string, namespaces []string) (string, string) {
	user, password := catalog.NewUserCredentials()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	Expect(err).ToNot(HaveOccurred())

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.GenerateResourceName("ruser", "test", user),
			Namespace: "epinio",
			Labels: map[string]string{
				"epinio.io/api-user-credentials": "true",
				"epinio.io/role":                 role,
			},
		},
		StringData: map[string]string{
			"username":   user,
			"password":   string(hashedPassword),
			"namespaces": strings.Join(namespaces, "\n"),
		},
	}

	secretTmpFile := catalog.NewTmpName("tmpUserFile") + `.json`
	file, err := os.Create(secretTmpFile)
	Expect(err).ToNot(HaveOccurred())

	err = json.NewEncoder(file).Encode(secret)
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(secretTmpFile)

	out, err := proc.Kubectl("apply", "-f", secretTmpFile)
	Expect(err).ToNot(HaveOccurred(), out)

	return user, password
}

// DeleteEpinioUser deletes the relevant Kubernetes secret if it exists.
func (m *Machine) DeleteEpinioUser(username string) error {
	username = names.GenerateResourceName("ruser", "test", username)
	out, err := proc.Kubectl("delete", "secret", "-n", "epinio", username, "--ignore-not-found")
	Expect(err).ToNot(HaveOccurred(), out)

	return nil
}
