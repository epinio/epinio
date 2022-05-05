package machine

import (
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateEpinioUser creates a new "user" BasicAuth Secret labeled as an Epinio User.
func (m *Machine) CreateEpinioUser(role string, namespaces []string) (string, string) {
	user, password := catalog.NewUserCredentials()

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Type: "BasicAuth",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "epinio-user-" + user,
			Namespace: "epinio",
			Labels: map[string]string{
				"epinio.suse.org/api-user-credentials": "true",
				"epinio.suse.org/role":                 role,
			},
		},
		StringData: map[string]string{
			"username":   user,
			"password":   password,
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
	out, err := proc.Kubectl("delete", "secret", "-n", "epinio", "epinio-user-"+username, "--ignore-not-found")
	Expect(err).ToNot(HaveOccurred(), out)

	return nil
}
