package acceptance_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/epinio/epinio/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

type WordpressApp struct {
	Name      string
	Dir       string
	SourceURL string
}

// CreateDir sets up a directory for a Wordpress application
func (w *WordpressApp) CreateDir() error {
	var err error
	if w.Dir, err = ioutil.TempDir("", "epinio-acceptance"); err != nil {
		return err
	}
	if out, err := helpers.RunProc("wget "+w.SourceURL, w.Dir, false); err != nil {
		return errors.Wrap(err, out)
	}
	if out, err := helpers.RunProc("tar xvf wordpress-*.tar.gz", w.Dir, false); err != nil {
		return errors.Wrap(err, out)
	}
	if out, err := helpers.RunProc("mv wordpress htdocs", w.Dir, false); err != nil {
		return errors.Wrap(err, out)
	}

	if out, err := helpers.RunProc("rm wordpress-*.tar.gz", w.Dir, false); err != nil {
		return errors.Wrap(err, out)
	}

	buildpackYaml := []byte(`
---
php:
  version: 7.3.x
  script: index.php
  webserver: nginx
  webdirectory: htdocs
`)
	if err := ioutil.WriteFile(path.Join(w.Dir, "buildpack.yml"), buildpackYaml, 0644); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(w.Dir, ".php.ini.d"), 0755); err != nil {
		return err
	}

	phpIni := []byte(`
extension=zlib
extension=mysqli
`)
	if err := ioutil.WriteFile(path.Join(w.Dir, ".php.ini.d", "extensions.ini"), phpIni, 0755); err != nil {
		return err
	}

	return nil
}

// Uri Finds the application ingress and returns the url to the app.
func (w *WordpressApp) AppURL() (string, error) {
	helpers.Kubectl(`get ingress  -n epinio-workloads --field-selector=metadata.name=` + w.Name + ` -o=jsonpath="{.items[0].spec['rules'][0]['host']}"`)
	host, err := helpers.Kubectl(`get ingress  -n epinio-workloads --field-selector=metadata.name=` + w.Name + ` -o=jsonpath="{.items[0].spec['rules'][0]['host']}"`)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://%s", host), nil
}

var _ = Describe("Wordpress", func() {
	var wordpress WordpressApp

	BeforeEach(func() {
		wordpress = WordpressApp{
			SourceURL: "https://wordpress.org/wordpress-5.6.1.tar.gz",
			Name:      newAppName(),
		}

		err := wordpress.CreateDir()
		Expect(err).ToNot(HaveOccurred())
	})

	It("can deploy Wordpress", func() {
		out, err := Epinio(fmt.Sprintf("apps push %s --verbosity 1", wordpress.Name), wordpress.Dir)
		Expect(err).ToNot(HaveOccurred(), out)

		out, err = Epinio("app list", "")
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(MatchRegexp(wordpress.Name + `.*\|.*1\/1.*\|.*`))

		appURL, err := wordpress.AppURL()
		Expect(err).ToNot(HaveOccurred())

		request, err := http.NewRequest("GET", appURL, nil)
		Expect(err).ToNot(HaveOccurred())
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		Eventually(func() int {
			resp, err := client.Do(request)
			Expect(err).ToNot(HaveOccurred())
			resp.Body.Close() // https://golang.org/pkg/net/http/#Client.Do

			return resp.StatusCode
		}, 5*time.Minute, 1*time.Second).Should(Equal(http.StatusOK))
	})
})
