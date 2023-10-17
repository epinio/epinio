// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apps_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/internal/names"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

type RailsApp struct {
	Name           string
	Namespace      string
	Dir            string
	SourceURL      string
	CredentialsEnc string
	MasterKey      string
}

func (r *RailsApp) CreateDir() error {
	var err error

	var tmpDir string
	if tmpDir, err = os.MkdirTemp("", "epinio-acceptance"); err != nil {
		return err
	}

	if out, err := proc.Run(tmpDir, false,
		"wget", r.SourceURL, "-O", "rails.tar"); err != nil {
		return errors.Wrap(err, out)
	}

	if out, err := proc.Run(tmpDir, false, "mkdir", "rails"); err != nil {
		return errors.Wrap(err, out)
	}

	if out, err := proc.Run(tmpDir, false,
		"tar", "xvf", "rails.tar", "-C", "rails", "--strip-components", "1"); err != nil {
		return errors.Wrap(err, out)
	}

	r.Dir = path.Join(tmpDir, "rails")

	if out, err := proc.Run(tmpDir, false, "rm", "rails.tar"); err != nil {
		return errors.Wrap(err, out)
	}

	if err := os.WriteFile(path.Join(r.Dir, "config", "credentials.yml.enc"), []byte(r.CredentialsEnc), 0644); err != nil {
		return errors.Wrap(err, "creating credentials file")
	}

	return nil
}

var _ = Describe("RubyOnRails", func() {
	var rails RailsApp
	var serviceName string
	var catalogName string
	var configurationName string
	var newHost string

	BeforeEach(func() {
		// Hardcode the contents of `config/credentials.yml.enc to avoid having to
		// call `rails` to do so. This should be decryptable with the masterKey
		// variable that follows.
		rails = RailsApp{
			Name:           catalog.NewAppName(),
			Namespace:      catalog.NewNamespaceName(),
			SourceURL:      "https://github.com/epinio/example-rails/tarball/main",
			CredentialsEnc: `uVPZWDUhuOVhjFhPhom5qL9dGAJqVOctoK8PZQpGp4i5rBnrcT7GiHFFAmPb3ZPSdAnW8sj00VlEECRem01LzI1pzfhg9TUGti6b2jyxiTxALVsDlmCg4V458jprpFfNJaAlK7RGRKp9oSNEI1DBliGX8aKTf6ye9wJV2AF+w4mdezj2xtsgN5lKhMN6YMFn8V/XNUC3cvmyEH6ot0Aj3N+BaiKXfTDJdaLqcr+awhMSNh0Es+vBLdYRvOgaMCGicKor/Oe0h8VkuVSIT0Ye08evYqoHkijKMH034T2M2rE5EhkKUzbK1YRhYPiPfHwoKYXviuarIuCZuR/q5WhVghc5YTRVUjFILWe5aLzrm9pCu0WweIDIDf4K7OGsQN07nY2a3974OR73qKEi1RCJGk+2dpn1c696f9ar--0GJc3grQhOubjNmy--+9a7S7qwSUi/ennPYg8XFg==`,
			MasterKey:      "75a74503267d5869281389d73cf8b90b",
		}
		env.SetupAndTargetNamespace(rails.Namespace)

		err := rails.CreateDir()
		Expect(err).ToNot(HaveOccurred())

		// Create the app
		out, err := env.Epinio("", "apps", "create", rails.Name)
		Expect(err).ToNot(HaveOccurred(), out)

		// Force node version used by buildpack to 16.x
		out, err = env.Epinio("", "apps", "env", "set", rails.Name, "BP_NODE_VERSION", "16.*")
		Expect(err).ToNot(HaveOccurred(), out)

		// Provide the expected RAILS_MASTER_KEY to buildpack and application
		out, err = env.Epinio("", "apps", "env", "set", rails.Name, "RAILS_MASTER_KEY", rails.MasterKey)
		Expect(err).ToNot(HaveOccurred(), out)

		// Create a custom service catalog
		serviceName = names.Truncate(catalog.NewServiceName(), 20)
		catalogName = names.Truncate(catalog.NewCatalogServiceName(), 20)

		out, err = proc.RunW("sed", "-i", "-e", fmt.Sprintf("s/myname/%s/", catalogName), testenv.TestAssetPath("my-postgresql-custom-svc.yaml"))
		Expect(err).ToNot(HaveOccurred(), out)

		out, err = proc.Kubectl("apply", "-f", testenv.TestAssetPath("my-postgresql-custom-svc.yaml"))
		Expect(err).ToNot(HaveOccurred(), out)

		// Create a database for Rails
		out, err = env.Epinio("", "service", "create", catalogName, serviceName)
		Expect(err).ToNot(HaveOccurred(), out)

		Eventually(func() string {
			out, _ := env.Epinio("", "service", "show", serviceName)
			return out
		}, "2m", "5s").Should(MatchRegexp("Status.*\\|.*deployed"))

		// Bind the database to app
		out, err = env.Epinio("", "service", "bind", serviceName, rails.Name)
		Expect(err).ToNot(HaveOccurred(), out)

		// See the configuration
		out, err = env.Epinio("", "configurations", "list")
		Expect(err).ToNot(HaveOccurred(), out)

		// Update the configuration
		configurationName = fmt.Sprintf("%s-postgresql",
			names.ServiceReleaseName(serviceName))
		newHost = fmt.Sprintf("%s.%s.svc.cluster.local", configurationName, rails.Namespace)

		out, err = env.Epinio("", "configurations", "update", configurationName,
			"--set", "host="+newHost,
			"--set", "port=5432",
			"--set", "username=myuser")
		Expect(err).ToNot(HaveOccurred(), out)

		// Change Rails database configuration to have the correct mount point in the access paths.
		// For 1.22 this was the configuration name
		// For 1.23+ it is the service name instead
		out, err = proc.Run(rails.Dir, false, "sed", "-i", fmt.Sprintf("s/mydb/%s/", serviceName),
			"config/database.yml")
		Expect(err).ToNot(HaveOccurred(), out)
	})

	AfterEach(func() {
		// Delete my custom service catalog
		out, err := proc.Kubectl("delete", "-f", testenv.TestAssetPath("my-postgresql-custom-svc.yaml"))
		Expect(err).ToNot(HaveOccurred(), out)
		out, err = proc.RunW("sed", "-i", "-e", fmt.Sprintf("s/%s/myname/", catalogName), testenv.TestAssetPath("my-postgresql-custom-svc.yaml"))
		Expect(err).ToNot(HaveOccurred(), out)

		env.DeleteApp(rails.Name)
		env.DeleteService(serviceName)
		env.DeleteNamespace(rails.Namespace)
	})

	It("can deploy Rails", func() {
		out, err := env.EpinioPush(rails.Dir, rails.Name,
			"--name", rails.Name,
			"--builder-image", "paketobuildpacks/builder:0.2.443-full")
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
		bodyBytes, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(string(bodyBytes)).To(MatchRegexp("Hello from Epinio!"))
	})
})
