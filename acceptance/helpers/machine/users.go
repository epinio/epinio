package machine

import (
	"fmt"
	"os"

	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/helpers"
)

// CreateEpinioUser creates a new "user" BasicAuth Secret labeled as an Epinio User.
func (m *Machine) CreateEpinioUser() (string, string) {
	user, password := catalog.NewUserCredentials()
	secretData := fmt.Sprintf(`apiVersion: v1
stringData:
  username: "%s"
  password: "%s"
kind: Secret
metadata:
  labels:
    epinio.suse.org/api-user-credentials: "true"
  name: epinio-user-%s
  namespace: epinio
type: BasicAuth
`, user, password, user)

	secretTmpFile := catalog.NewTmpName("tmpUserFile") + `.yaml`
	err := os.WriteFile(secretTmpFile, []byte(secretData), 0600)
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(secretTmpFile)

	out, err := helpers.Kubectl("apply", "-f", secretTmpFile)
	Expect(err).ToNot(HaveOccurred(), out)

	return user, password
}

func (m *Machine) DeleteEpinioUser(username string) error {
	out, err := helpers.Kubectl("delete", "secret", "-n", "epinio", "epinio-user-"+username)
	Expect(err).ToNot(HaveOccurred(), out)

	return nil
}
