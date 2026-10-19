// Copyright © 2021 - 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package harbor installs Harbor for EPINIO-823 acceptance tests.
//
// Scope is Harbor-only (not Docker Hub). Harbor is exposed on an ingress
// hostname harbor.<domain> with a dedicated TLS certificate so Epinio verifies
// the registry with containerregistry.certificateSecret (not the in-cluster
// .svc.cluster.local path that skips TLS verification).
package harbor

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	Namespace                 = "harbor"
	ReleaseName               = "harbor"
	AdminUser                 = "admin"
	ProjectName               = "epinio"
	TLSSecretName             = "harbor-ingress-tls"
	ExternalRegistryTLSSecret = "epinio-external-registry-tls"
	HelmChartVersion          = "1.16.2"
	defaultAdminPassword      = "HarborTest12345"
)

// AdminPassword returns the Harbor admin password (HARBOR_ADMIN_PASSWORD or default).
func AdminPassword() string {
	if p := os.Getenv("HARBOR_ADMIN_PASSWORD"); p != "" {
		return p
	}
	return defaultAdminPassword
}

// Host returns the ingress hostname for Harbor on the given Epinio system domain.
func Host(domain string) string {
	return "harbor." + domain
}

// Install deploys Harbor behind ingress at harbor.<domain>, creates the epinio
// project, and returns the registry host (no scheme).
func Install(domain string) string {
	GinkgoHelper()

	Expect(domain).NotTo(BeEmpty(), "domain is required for Harbor ingress hostname")
	host := Host(domain)
	password := AdminPassword()
	tmpDir := GinkgoT().TempDir()

	By("adding the Harbor helm repository")
	out, err := proc.RunW("helm", "repo", "add", "harbor", "https://helm.goharbor.io", "--force-update")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "repo", "update", "harbor")
	Expect(err).NotTo(HaveOccurred(), out)

	By("creating the Harbor namespace and TLS secret for " + host)
	nsFile := filepath.Join(tmpDir, "ns.yaml")
	Expect(os.WriteFile(nsFile, []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: "+Namespace+"\n"), 0600)).To(Succeed())
	out, err = proc.RunW("kubectl", "apply", "-f", nsFile)
	Expect(err).NotTo(HaveOccurred(), out)

	createTLSSecret(tmpDir, host)

	By("installing Harbor with ingress")
	out, err = proc.RunW("helm", "upgrade", "--install", ReleaseName, "harbor/harbor",
		"--version", HelmChartVersion,
		"--namespace", Namespace,
		"--set", "expose.type=ingress",
		"--set", "expose.tls.enabled=true",
		"--set", "expose.tls.certSource=secret",
		"--set", "expose.tls.secret.secretName="+TLSSecretName,
		"--set", "expose.ingress.hosts.core="+host,
		"--set", "externalURL=https://"+host,
		"--set", "harborAdminPassword="+password,
		"--set", "persistence.enabled=false",
		"--timeout", "15m",
		"--wait",
	)
	Expect(err).NotTo(HaveOccurred(), out)

	By("waiting for Harbor core to be ready")
	Eventually(func() string {
		out, _ := proc.RunW("kubectl", "get", "pods", "-n", Namespace,
			"-l", "component=core",
			"-o", "jsonpath={.items[0].status.phase}")
		return out
	}, "8m", "5s").Should(Equal("Running"))

	By("creating the Harbor project " + ProjectName)
	createProject(tmpDir, host, password)

	return host
}

// Uninstall removes the Harbor release, namespace, and Epinio registry CA secret.
func Uninstall() {
	GinkgoHelper()

	By("uninstalling Harbor")
	_, _ = proc.RunW("helm", "uninstall", ReleaseName, "--namespace", Namespace)
	_, _ = proc.RunW("kubectl", "delete", "namespace", Namespace, "--ignore-not-found", "--wait=false")
	DeleteExternalRegistryTLS()
}

// DeleteExternalRegistryTLS removes the CA secret copied into the epinio namespace.
func DeleteExternalRegistryTLS() {
	_, _ = proc.RunW("kubectl", "delete", "secret", ExternalRegistryTLSSecret,
		"-n", "epinio", "--ignore-not-found")
}

// CopyCAToEpinio creates epinio-external-registry-tls from Harbor's ingress TLS cert.
func CopyCAToEpinio() {
	GinkgoHelper()

	By("creating the epinio namespace for the registry CA secret")
	tmpDir := GinkgoT().TempDir()
	nsFile := filepath.Join(tmpDir, "epinio-ns.yaml")
	Expect(os.WriteFile(nsFile, []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: epinio\n"), 0600)).To(Succeed())
	out, err := proc.RunW("kubectl", "apply", "-f", nsFile)
	Expect(err).NotTo(HaveOccurred(), out)

	By("copying Harbor TLS cert into " + ExternalRegistryTLSSecret)
	out, err = proc.RunW("kubectl", "get", "secret", TLSSecretName,
		"-n", Namespace,
		"-o", "jsonpath={.data.tls\\.crt}")
	Expect(err).NotTo(HaveOccurred(), out)
	Expect(out).NotTo(BeEmpty())

	certPEM, err := base64.StdEncoding.DecodeString(out)
	Expect(err).NotTo(HaveOccurred())
	Expect(string(certPEM)).To(ContainSubstring("BEGIN CERTIFICATE"))

	certFile := filepath.Join(tmpDir, "tls.crt")
	Expect(os.WriteFile(certFile, certPEM, 0600)).To(Succeed())

	_, _ = proc.RunW("kubectl", "delete", "secret", ExternalRegistryTLSSecret,
		"-n", "epinio", "--ignore-not-found")
	out, err = proc.RunW("kubectl", "create", "secret", "generic", ExternalRegistryTLSSecret,
		"-n", "epinio",
		"--from-file=tls.crt="+certFile)
	Expect(err).NotTo(HaveOccurred(), out)
}

// ExpectAppChartRepo lists Harbor repositories in the epinio project and asserts
// that a repo for the named app chart exists (epinio_appchart_<name>).
func ExpectAppChartRepo(host, chartName string) {
	GinkgoHelper()

	password := AdminPassword()
	repoPrefix := "epinio_appchart_" + chartName
	tmpDir := GinkgoT().TempDir()

	script := fmt.Sprintf(`set -e
body=$(curl -sk -u '%[1]s:%[2]s' \
  'https://%[3]s/api/v2.0/projects/%[4]s/repositories?page_size=100' || true)
echo "$body"
echo "$body" | grep -q '%[5]s'
`, AdminUser, password, host, ProjectName, repoPrefix)

	runCurlScript(tmpDir, "harbor-expect-repo", script)
}

func createTLSSecret(tmpDir, host string) {
	GinkgoHelper()

	keyFile := filepath.Join(tmpDir, "tls.key")
	certFile := filepath.Join(tmpDir, "tls.crt")

	out, err := proc.RunW("openssl", "req", "-x509", "-nodes",
		"-newkey", "rsa:2048",
		"-keyout", keyFile,
		"-out", certFile,
		"-days", "365",
		"-subj", "/CN="+host,
		"-addext", "subjectAltName=DNS:"+host,
	)
	Expect(err).NotTo(HaveOccurred(), out)

	_, _ = proc.RunW("kubectl", "delete", "secret", TLSSecretName,
		"-n", Namespace, "--ignore-not-found")
	out, err = proc.RunW("kubectl", "create", "secret", "tls", TLSSecretName,
		"-n", Namespace,
		"--cert="+certFile,
		"--key="+keyFile)
	Expect(err).NotTo(HaveOccurred(), out)
}

func createProject(tmpDir, host, password string) {
	GinkgoHelper()

	script := fmt.Sprintf(`set -e
for i in $(seq 1 60); do
  if curl -sk -u '%[1]s:%[2]s' 'https://%[3]s/api/v2.0/systeminfo' >/dev/null; then
    break
  fi
  sleep 5
done
code=$(curl -sk -o /tmp/harbor-project.out -w '%%{http_code}' -u '%[1]s:%[2]s' \
  -H 'Content-Type: application/json' \
  -X POST 'https://%[3]s/api/v2.0/projects' \
  -d '{"project_name":"%[4]s","public":false}' || true)
if [ "$code" != "201" ] && [ "$code" != "409" ]; then
  echo "create project failed: HTTP $code"
  cat /tmp/harbor-project.out || true
  exit 1
fi
`, AdminUser, password, host, ProjectName)

	runCurlScript(tmpDir, "harbor-create-project", script)
}

func runCurlScript(tmpDir, name, script string) {
	GinkgoHelper()

	cmYAML := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  run.sh: |
%s
`, name, Namespace, indentLines(script, "    "))
	cmFile := filepath.Join(tmpDir, name+"-cm.yaml")
	Expect(os.WriteFile(cmFile, []byte(cmYAML), 0600)).To(Succeed())
	out, err := proc.RunW("kubectl", "apply", "-f", cmFile)
	Expect(err).NotTo(HaveOccurred(), out)

	_, _ = proc.RunW("kubectl", "delete", "pod", name,
		"-n", Namespace, "--ignore-not-found", "--wait=false")

	podYAML := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  restartPolicy: Never
  containers:
    - name: curl
      image: curlimages/curl:8.5.0
      command: ["sh", "/scripts/run.sh"]
      volumeMounts:
        - name: scripts
          mountPath: /scripts
  volumes:
    - name: scripts
      configMap:
        name: %s
        defaultMode: 0755
`, name, Namespace, name)
	podFile := filepath.Join(tmpDir, name+"-pod.yaml")
	Expect(os.WriteFile(podFile, []byte(podYAML), 0600)).To(Succeed())
	out, err = proc.RunW("kubectl", "apply", "-f", podFile)
	Expect(err).NotTo(HaveOccurred(), out)

	Eventually(func() string {
		out, _ := proc.RunW("kubectl", "get", "pod", name,
			"-n", Namespace, "-o", "jsonpath={.status.phase}")
		return out
	}, 3*time.Minute, 5*time.Second).Should(Or(Equal("Succeeded"), Equal("Failed")))

	out, err = proc.RunW("kubectl", "get", "pod", name,
		"-n", Namespace, "-o", "jsonpath={.status.phase}")
	Expect(err).NotTo(HaveOccurred(), out)
	if out != "Succeeded" {
		logs, _ := proc.RunW("kubectl", "logs", name, "-n", Namespace)
		Fail(name + " pod failed:\n" + logs)
	}

	_, _ = proc.RunW("kubectl", "delete", "pod", name, "-n", Namespace, "--ignore-not-found")
	_, _ = proc.RunW("kubectl", "delete", "configmap", name, "-n", Namespace, "--ignore-not-found")
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
