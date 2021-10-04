package apps_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

type RailsApp struct {
	Name           string
	Org            string
	Dir            string
	SourceURL      string
	CredentialsEnc string
	MasterKey      string
}

func (r *RailsApp) CreateDir() error {
	var err error

	var tmpDir string
	if tmpDir, err = ioutil.TempDir("", "epinio-acceptance"); err != nil {
		return err
	}

	if out, err := helpers.RunProc(tmpDir, false,
		"wget", r.SourceURL, "-O", "rails.tar"); err != nil {
		return errors.Wrap(err, out)
	}

	if out, err := helpers.RunProc(tmpDir, false, "mkdir", "rails"); err != nil {
		return errors.Wrap(err, out)
	}

	if out, err := helpers.RunProc(tmpDir, false,
		"tar", "xvf", "rails.tar", "-C", "rails", "--strip-components", "1"); err != nil {
		return errors.Wrap(err, out)
	}

	r.Dir = path.Join(tmpDir, "rails")

	if out, err := helpers.RunProc(tmpDir, false, "rm", "rails.tar"); err != nil {
		return errors.Wrap(err, out)
	}

	if err := ioutil.WriteFile(path.Join(r.Dir, "config", "credentials.yml.enc"), []byte(r.CredentialsEnc), 0644); err != nil {
		return errors.Wrap(err, "creating credentials file")
	}

	return nil
}

var _ = Describe("RubyOnRails", func() {
	var rails RailsApp
	var serviceName string

	BeforeEach(func() {
		// Hardcode the contents of `config/credentials.yml.enc to avoid having to
		// call `rails` to do so. This should be decryptable with the masterKey
		// variable that follows.
		rails = RailsApp{
			Name:           catalog.NewAppName(),
			Org:            catalog.NewOrgName(),
			SourceURL:      "https://github.com/epinio/example-rails/tarball/main",
			CredentialsEnc: `uVPZWDUhuOVhjFhPhom5qL9dGAJqVOctoK8PZQpGp4i5rBnrcT7GiHFFAmPb3ZPSdAnW8sj00VlEECRem01LzI1pzfhg9TUGti6b2jyxiTxALVsDlmCg4V458jprpFfNJaAlK7RGRKp9oSNEI1DBliGX8aKTf6ye9wJV2AF+w4mdezj2xtsgN5lKhMN6YMFn8V/XNUC3cvmyEH6ot0Aj3N+BaiKXfTDJdaLqcr+awhMSNh0Es+vBLdYRvOgaMCGicKor/Oe0h8VkuVSIT0Ye08evYqoHkijKMH034T2M2rE5EhkKUzbK1YRhYPiPfHwoKYXviuarIuCZuR/q5WhVghc5YTRVUjFILWe5aLzrm9pCu0WweIDIDf4K7OGsQN07nY2a3974OR73qKEi1RCJGk+2dpn1c696f9ar--0GJc3grQhOubjNmy--+9a7S7qwSUi/ennPYg8XFg==`,
			MasterKey:      "75a74503267d5869281389d73cf8b90b",
		}
		env.SetupAndTargetOrg(rails.Org)

		err := rails.CreateDir()
		Expect(err).ToNot(HaveOccurred())

		// Create the app
		out, err := env.Epinio("", "apps", "create", rails.Name)
		Expect(err).ToNot(HaveOccurred(), out)

		// Set the RAILS_MASTER_KEY env variable
		out, err = env.Epinio("", "apps", "env", "set", rails.Name, "RAILS_MASTER_KEY", rails.MasterKey)
		Expect(err).ToNot(HaveOccurred(), out)

		// Create a database for Rails
		out, err = proc.Run("", false, "helm", "repo", "add", "bitnami", "https://charts.bitnami.com/bitnami")
		Expect(err).ToNot(HaveOccurred(), out)

		// Update helm repos
		out, err = proc.Run("", false, "helm", "repo", "update")
		Expect(err).ToNot(HaveOccurred(), out)

		serviceName = catalog.NewServiceName()
		out, err = proc.Run("", false,
			"helm", "install", serviceName, "bitnami/postgresql", "--version", "10.12.0", "-n", rails.Org,
			"--set", "postgresqlDatabase=production",
			"--set", "postgresqlUsername=myuser",
			"--set", "postgresqlPassword=mypassword",
			"--set", "volumePermissions.enabled=true")
		Expect(err).ToNot(HaveOccurred(), out)

		out, err = env.Epinio("", "service", "create", serviceName,
			"username", "myuser",
			"password", "mypassword",
			"host", fmt.Sprintf("%s-postgresql.%s.svc.cluster.local", serviceName, rails.Org),
			"port", "5432",
		)
		Expect(err).ToNot(HaveOccurred(), out)

		// Change Rails database configuration to match the service name
		out, err = proc.Run(rails.Dir, false, "sed", "-i", fmt.Sprintf("s/mydb/%s/", serviceName), "config/database.yml")
		Expect(err).ToNot(HaveOccurred(), out)
	})

	AfterEach(func() {
		env.DeleteServiceUnbind(serviceName)
		out, err := proc.Run("", false, "helm", "delete", serviceName, "-n", rails.Org)
		Expect(err).ToNot(HaveOccurred(), out)

		env.DeleteApp(rails.Name)
	})

	It("can deploy Rails", func() {
		out, err := env.Epinio(rails.Dir, "apps", "push", rails.Name, "-b", serviceName)
		Expect(err).ToNot(HaveOccurred(), out)

		route := testenv.AppRouteFromOutput(out)
		Expect(route).ToNot(BeEmpty())

		Eventually(func() int {
			resp, err := env.Curl("GET", route, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			return resp.StatusCode
		}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

		resp, err := env.Curl("GET", route, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(string(bodyBytes)).To(MatchRegexp("Hello from Epinio!"))
	})
})
