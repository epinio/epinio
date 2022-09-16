package auth

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/helpers"

	. "github.com/onsi/gomega"
)

type User struct {
	username   string
	password   string
	role       string
	workspaces []string
}

var users = []User{
	{"admin@epinio.io", "password", "admin", []string{""}},
	{"user1@epinio.io", "password", "user", []string{""}},
	{"user2@epinio.io", "password", "user", []string{""}},
}

func InitUsers(env *testenv.EpinioEnv, apiURL string) {
	createDexUsers(apiURL)

	restartDex()

	createEpinioUsersAndLogin(env, apiURL)
}

func createDexUsers(apiURL string) {
	domain := strings.TrimPrefix(apiURL, "https://epinio.")

	// get clientSecret
	out, err := proc.Kubectl("get", "secret", "-n", "epinio", "dex-client-secret", "--template={{.data.clientSecret}}")
	Expect(err).ToNot(HaveOccurred())
	clientSecret, err := base64.StdEncoding.DecodeString(out)
	Expect(err).ToNot(HaveOccurred())

	dexConfSecretTemplate, err := os.ReadFile(testenv.TestAssetPath("dex-conf.yaml"))
	Expect(err).ToNot(HaveOccurred())

	dexConf := fmt.Sprintf(string(dexConfSecretTemplate), domain, clientSecret)
	filePath, err := helpers.CreateTmpFile(dexConf)
	Expect(err).ToNot(HaveOccurred())

	// apply conf
	out, err = proc.Kubectl("apply", "-f", filePath)
	Expect(err).ToNot(HaveOccurred(), out)
}

func restartDex() {
	// sometimes this fails for empty patch
	_, _ = proc.Kubectl("rollout", "restart", "-n", "epinio", "deployment/dex")
	out, err := proc.Kubectl("rollout", "status", "-n", "epinio", "deployment/dex")
	Expect(err).ToNot(HaveOccurred(), out)

	time.Sleep(10 * time.Second)
}

func createEpinioUsersAndLogin(env *testenv.EpinioEnv, apiURL string) {
	// cleanup
	out, err := proc.Kubectl("delete", "secrets", "-n", "epinio", "-l", "epinio.io/api-user-credentials")
	Expect(err).ToNot(HaveOccurred(), out)

	for _, user := range users {
		env.CreateEpinioUserWithUsernameAndPassword(user.username, user.password, user.role, user.workspaces)

		token, err := GetToken(apiURL, user.username, user.password)
		Expect(err).NotTo(HaveOccurred())
		env.EpinioTokenMap[user.username] = token
	}
}
