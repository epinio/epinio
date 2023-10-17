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
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

type WordpressApp struct {
	Name      string
	Namespace string
	Dir       string
	SourceURL string
}

// CreateDir sets up a directory for a Wordpress application
func (w *WordpressApp) CreateDir() error {
	var err error
	if w.Dir, err = os.MkdirTemp("", "epinio-acceptance"); err != nil {
		return err
	}
	if out, err := proc.Run(w.Dir, false, "wget", w.SourceURL); err != nil {
		return errors.Wrap(err, out)
	}

	tarPaths, err := filepath.Glob(w.Dir + "/wordpress-*.tar.gz")
	if err != nil {
		return err
	}

	if out, err := proc.Run(w.Dir, false, "tar", append([]string{"xvf"}, tarPaths...)...); err != nil {
		return errors.Wrap(err, out)
	}

	if out, err := proc.Run("", false, "rm", tarPaths...); err != nil {
		return errors.Wrap(err, out)
	}

	if err := os.MkdirAll(path.Join(w.Dir, ".php.ini.d"), 0755); err != nil {
		return err
	}

	phpIni := []byte(`
extension=zlib
extension=mysqli
`)
	if err := os.WriteFile(path.Join(w.Dir, ".php.ini.d", "extensions.ini"), phpIni, 0755); err != nil {
		return err
	}

	return nil
}

// AppURL Finds the application ingress and returns the url to the app.
// If more than one route is specified for the app, it will return the first
// one alphabetically.
func (w *WordpressApp) AppURL() (string, error) {
	out, err := proc.Kubectl("get", "ingress",
		"--namespace", w.Namespace,
		"--selector", "app.kubernetes.io/name="+w.Name,
		"-o", "jsonpath={.items[*].spec.rules[*].host}")
	if err != nil {
		return "", err
	}
	hosts := strings.Split(out, " ")
	sort.Strings(hosts)

	return fmt.Sprintf("https://%s", hosts[0]), nil
}

var _ = Describe("Wordpress", func() {
	var namespace string
	var wordpress WordpressApp

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		wordpress = WordpressApp{
			SourceURL: "https://wordpress.org/wordpress-5.6.1.tar.gz",
			Name:      catalog.NewAppName(),
			Namespace: namespace,
		}

		env.SetupAndTargetNamespace(namespace)

		err := wordpress.CreateDir()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		out, err := env.Epinio("", "apps", "delete", wordpress.Name)
		Expect(err).ToNot(HaveOccurred(), out)

		err = os.RemoveAll(wordpress.Dir)
		Expect(err).ToNot(HaveOccurred())
		env.DeleteNamespace(namespace)
	})

	It("can deploy Wordpress", func() {
		out, err := env.EpinioPush(wordpress.Dir, wordpress.Name, "--name", wordpress.Name,
			"--builder-image", "paketobuildpacks/builder:0.2.443-full",
			"-e", "BP_PHP_WEB_DIR=wordpress",
			"-e", "BP_PHP_VERSION=8.0.x",
			"-e", "BP_PHP_SERVER=nginx")
		Expect(err).ToNot(HaveOccurred(), out)

		out, err = env.Epinio("", "app", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(MatchRegexp(wordpress.Name + `.*\|.*1\/1.*\|.*`))

		appURL, err := wordpress.AppURL()
		Expect(err).ToNot(HaveOccurred())

		request, err := http.NewRequest("GET", appURL, nil)
		Expect(err).ToNot(HaveOccurred())
		client := env.Client()
		Eventually(func() int {
			resp, err := client.Do(request)
			ExpectWithOffset(1, err).ToNot(HaveOccurred())
			resp.Body.Close() // https://golang.org/pkg/net/http/#Client.Do

			return resp.StatusCode
		}, 5*time.Minute, 1*time.Second).Should(Equal(http.StatusOK))
	})
})
