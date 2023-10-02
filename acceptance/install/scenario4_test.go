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

package install_test

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/internal/names"
)

// This test uses AWS route53 to update the system domain's records
var _ = Describe("<Scenario4> EKS, epinio-ca, on S3 storage", func() {
	var (
		flags           []string
		epinioHelper    epinio.Epinio
		appName         = catalog.NewAppName()
		serviceName     = catalog.NewServiceName()
		loadbalancer    string
		domain          string
		zoneID          string
		accessKeyID     string
		secretAccessKey string
		extraEnvName    string
		extraEnvValue   string
		name_exists     bool
		value_exists    bool
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		domain = os.Getenv("EPINIO_SYSTEM_DOMAIN")
		Expect(domain).ToNot(BeEmpty())

		zoneID = os.Getenv("AWS_ZONE_ID")
		Expect(zoneID).ToNot(BeEmpty())

		accessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
		Expect(accessKeyID).ToNot(BeEmpty())

		secretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		Expect(secretAccessKey).ToNot(BeEmpty())

		flags = []string{
			"--set", "server.disableTracking=true", // disable tracking during tests
			"--set", "global.domain=" + domain,
			"--set", "minio.enabled=false",
			"--set", "s3.useSSL=true",
			"--set", "s3.bucket=epinio-ci",
			"--set", "s3.endpoint=s3.eu-central-1.amazonaws.com",
			"--set", "s3.region=eu-central-1",
			"--set", "s3.accessKeyID=" + accessKeyID,
			"--set", "s3.secretAccessKey=" + secretAccessKey,
		}

		extraEnvName, name_exists = os.LookupEnv("EXTRAENV_NAME")
		extraEnvValue, value_exists = os.LookupEnv("EXTRAENV_VALUE")
		if name_exists && value_exists {
			flags = append(flags, "--set", "extraEnv[0].name="+extraEnvName, "--set-string", "extraEnv[0].value="+extraEnvValue)
		}
	})

	AfterEach(func() {
		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("Installs with loadbalancer IP, custom domain and pushes an app with env vars", func() {
		By("Checking LoadBalancer IP", func() {
			// Ensure that Nginx LB is not in Pending state anymore, could take time
			Eventually(func() string {
				out, err := proc.RunW("kubectl", "get", "svc", "-n", "ingress-nginx", "ingress-nginx-controller", "--no-headers")
				Expect(err).NotTo(HaveOccurred(), out)
				return out
			}, "4m", "2s").ShouldNot(ContainSubstring("<pending>"))

			out, err := proc.RunW("kubectl", "get", "service", "-n", "ingress-nginx", "ingress-nginx-controller", "-o", "json")
			Expect(err).NotTo(HaveOccurred(), out)

			// Check that an IP address for LB is configured
			status := &testenv.LoadBalancerHostname{}
			err = json.Unmarshal([]byte(out), status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.Status.LoadBalancer.Ingress).To(HaveLen(1))
			loadbalancer = status.Status.LoadBalancer.Ingress[0].Hostname
			Expect(loadbalancer).ToNot(BeEmpty())
		})

		By("Updating DNS Entries", func() {
			change := route53.CNAME(domain, loadbalancer, "UPSERT")
			out, err := route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)

			change = route53.CNAME("*."+domain, loadbalancer, "UPSERT")
			out, err = route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		// Check that DNS entry is correctly propagated
		By("Checking that DNS entry is correctly propagated", func() {
			Eventually(func() string {
				out, err := route53.TestDnsAnswer(zoneID, domain, "CNAME")
				Expect(err).NotTo(HaveOccurred(), out)

				answer := &route53.DNSAnswer{}
				err = json.Unmarshal([]byte(out), answer)
				Expect(err).NotTo(HaveOccurred())
				if len(answer.RecordData) == 0 {
					return ""
				}
				return answer.RecordData[0]
			}, "5m", "2s").Should(Equal(loadbalancer + ".")) // CNAME ends with a '.'
		})

		// Workaround to (try to!) ensure that the DNS is really propagated!
		time.Sleep(3 * time.Minute)

		By("Installing Epinio", func() {
			out, err := epinioHelper.Install(flags...)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("STATUS: deployed"))

			out, err = testenv.PatchEpinio()
			Expect(err).ToNot(HaveOccurred(), out)
		})

		By("Allow internal HTTP registry on EKS 1.24+", func() {
			out, err := proc.Run(testenv.Root(), true, "kubectl", "apply", "-f", "./scripts/eks-cri-allow-http-registries.yaml")
			Expect(err).ToNot(HaveOccurred(), out)
			out, err = proc.Kubectl("wait", "--for=condition=complete", "job/setup-cri")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		By("Connecting to Epinio", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("login", "-u", "admin", "-p", "password", "--trust-ca", "https://epinio."+domain)
				return out
			}, "2m", "5s").Should(ContainSubstring("Login successful"))
		})

		By("Checking Epinio info command", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("info")
				return out
			}, "2m", "2s").Should(ContainSubstring("Epinio Server Version:"))
		})

		By("Targeting workspace namespace", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("target", "workspace")
				return out
			}, "2m", "2s").Should(ContainSubstring("Namespace targeted."))
		})

		By("Creating the application", func() {
			out, err := epinioHelper.Run("apps", "create", appName)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Deploying a database with service", func() {
			out, err := epinioHelper.Run("service", "create", "mysql-dev", serviceName)
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, _ := epinioHelper.Run("service", "show", serviceName)
				return out
			}, "2m", "5s").Should(MatchRegexp("Status.*\\|.*deployed"))
		})

		By("Bind the database to the app", func() {
			out, err := epinioHelper.Run("service", "bind", serviceName, appName)
			Expect(err).ToNot(HaveOccurred(), out)

			chart := names.ServiceReleaseName(serviceName)

			Eventually(func() string {
				out, err := epinioHelper.Run("app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "2m", "5s").Should(MatchRegexp("Bound Configurations.*\\|.*%s", chart))
		})

		By("Pushing an app with Env vars", func() {
			out, err := epinioHelper.Run("apps", "env", "set", appName, "MYVAR", "myvalue")
			Expect(err).ToNot(HaveOccurred(), out)

			out, err = epinioHelper.Run("push",
				"--name", appName,
				"--path", testenv.AssetPath("sample-app"))
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, err := proc.Kubectl("get", "deployments",
					"-l", fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", appName, testenv.DefaultWorkspace),
					"--namespace", testenv.DefaultWorkspace,
					"-o", "jsonpath={.items[].spec.template.spec.containers[0].env}")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}).Should(MatchRegexp("MYVAR"))
		})

		By("Delete an app", func() {
			out, err := epinioHelper.Run("apps", "delete", appName)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Or(ContainSubstring("Applications Removed")))
		})
	})
})
