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

	// k3d / loopback: patch DNS + kubelet registry trust before Harbor API / pulls.
	// Opt-in so RKE self-hosted CI (real node IPs) is unchanged.
	if os.Getenv("HARBOR_PATCH_INGRESS_DNS") == "1" {
		EnsureInClusterDNS(host)
		EnsureNodeRegistryTrust(host, password)
	}

	By("creating the Harbor project " + ProjectName)
	createProject(tmpDir, host, password)

	return host
}

const ingressNginxDNS = "ingress-nginx-controller.ingress-nginx.svc.cluster.local"

// EnsureInClusterDNS makes harborHost resolve inside the cluster to the ingress
// controller (not public sslip → 127.0.0.1, which pods cannot use).
//
// Strategy (k3d-safe):
//  1. CoreDNS "rewrite name exact" — works alongside k3d's existing hosts plugin
//     (a second hosts{} block crashes CoreDNS).
//  2. Node /etc/hosts on the k3d server — kubelet does not use CoreDNS.
//  3. Verify with an in-cluster nslookup before continuing.
func EnsureInClusterDNS(harborHost string) {
	GinkgoHelper()

	By("rewriting CoreDNS so " + harborHost + " → " + ingressNginxDNS)
	ingressIP := ingressControllerClusterIP()
	Expect(ingressIP).NotTo(BeEmpty(), "ingress controller ClusterIP not found")

	corefile, err := proc.RunW("kubectl", "-n", "kube-system", "get", "configmap", "coredns",
		"-o", "jsonpath={.data.Corefile}")
	Expect(err).NotTo(HaveOccurred(), corefile)
	Expect(corefile).NotTo(BeEmpty())

	rewriteLine := fmt.Sprintf("    rewrite name exact %s %s", harborHost, ingressNginxDNS)
	if !strings.Contains(corefile, rewriteLine) {
		patched := injectCoreDNSRewrite(corefile, rewriteLine)
		tmpDir := GinkgoT().TempDir()
		coreFilePath := filepath.Join(tmpDir, "Corefile")
		Expect(os.WriteFile(coreFilePath, []byte(patched), 0600)).To(Succeed())

		out, err := proc.RunW("sh", "-c",
			"kubectl -n kube-system create configmap coredns --from-file=Corefile="+coreFilePath+
				" -o yaml --dry-run=client | kubectl apply -f -")
		Expect(err).NotTo(HaveOccurred(), out)

		_, _ = proc.RunW("kubectl", "-n", "kube-system", "rollout", "restart", "deployment/coredns")
		out, err = proc.RunW("kubectl", "-n", "kube-system", "rollout", "status",
			"deployment/coredns", "--timeout=90s")
		Expect(err).NotTo(HaveOccurred(), out)
	}

	By("seeding k3d node /etc/hosts for kubelet image pulls")
	entry := ingressIP + " " + harborHost
	out, err := proc.RunW("sh", "-c", fmt.Sprintf(`
set -e
entry=%q
server=$(docker ps --filter name=k3d-epinio-acceptance-server-0 -q | head -n1)
test -n "$server"
docker exec -u root "$server" sh -c "grep -qF \"$entry\" /etc/hosts || echo \"$entry\" >> /etc/hosts"
`, entry))
	Expect(err).NotTo(HaveOccurred(), out)

	By("verifying in-cluster DNS for " + harborHost)
	Eventually(func() string {
		_, _ = proc.RunW("kubectl", "delete", "pod", "harbor-dns-check",
			"--ignore-not-found", "--wait=true")
		out, _ := proc.RunW("kubectl", "run", "harbor-dns-check", "--rm", "-i", "--restart=Never",
			"--image=busybox:1.36", "--",
			"nslookup", harborHost)
		return out
	}, "2m", "5s").Should(And(
		ContainSubstring(ingressIP),
		// Answer must not be loopback (query name itself contains "127.0.0.1" in sslip hosts).
		Not(MatchRegexp(`(?m)^Address:.*\b127\.0\.0\.1\b`)),
	))
}

// EnsureNodeRegistryTrust configures k3s containerd on the k3d server so kubelet
// can pull app images from Harbor (self-signed TLS + basic auth).
//
// Staging already trusts Harbor via containerregistry.certificateSecret; kubelet
// does not — without this, helm deploy hangs on ImagePullBackOff until atomic
// timeout. Opt-in via HARBOR_PATCH_INGRESS_DNS (same as EnsureInClusterDNS).
func EnsureNodeRegistryTrust(harborHost, password string) {
	GinkgoHelper()

	By("configuring k3s registries.yaml so kubelet trusts Harbor TLS")
	certB64, err := proc.RunW("kubectl", "get", "secret", TLSSecretName,
		"-n", Namespace,
		"-o", "jsonpath={.data.tls\\.crt}")
	Expect(err).NotTo(HaveOccurred(), certB64)
	Expect(certB64).NotTo(BeEmpty())

	tmpDir := GinkgoT().TempDir()
	certPath := filepath.Join(tmpDir, "harbor-ca.crt")
	certPEM, err := base64.StdEncoding.DecodeString(strings.TrimSpace(certB64))
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(certPath, certPEM, 0600)).To(Succeed())

	// Keep the docker.io mirror from acceptance-cluster-setup.sh and add Harbor.
	registries := fmt.Sprintf(`mirrors:
  "docker.io":
    endpoint:
      - "http://epinio-acceptance-registry-mirror:5000"
  %q:
    endpoint:
      - "https://%s"
configs:
  %q:
    auth:
      username: %q
      password: %q
    tls:
      ca_file: /etc/rancher/k3s/harbor-ca.crt
`, harborHost, harborHost, harborHost, AdminUser, password)
	regPath := filepath.Join(tmpDir, "registries.yaml")
	Expect(os.WriteFile(regPath, []byte(registries), 0600)).To(Succeed())

	out, err := proc.RunW("sh", "-c", fmt.Sprintf(`
set -e
server=$(docker ps --filter name=k3d-epinio-acceptance-server-0 -q | head -n1)
test -n "$server"
docker cp %q "$server":/etc/rancher/k3s/harbor-ca.crt
docker cp %q "$server":/etc/rancher/k3s/registries.yaml
# k3s reloads registry config on process restart (k3d keeps the container up).
docker exec -u root "$server" sh -c 'killall k3s 2>/dev/null || pkill k3s 2>/dev/null || true'
`, certPath, regPath))
	Expect(err).NotTo(HaveOccurred(), out)

	By("waiting for k3s node to become Ready after registries.yaml reload")
	Eventually(func() string {
		out, _ := proc.RunW("kubectl", "get", "nodes",
			"-o", "jsonpath={.items[0].status.conditions[?(@.type==\"Ready\")].status}")
		return strings.TrimSpace(out)
	}, "3m", "5s").Should(Equal("True"))
}

// injectCoreDNSRewrite inserts a rewrite line before the first "forward" directive.
// Does not add a hosts{} block (k3d already has one).
func injectCoreDNSRewrite(corefile, rewriteLine string) string {
	for _, marker := range []string{"\n    forward ", "\n    cache "} {
		if idx := strings.Index(corefile, marker); idx >= 0 {
			return corefile[:idx] + "\n" + rewriteLine + corefile[idx:]
		}
	}
	return corefile + "\n" + rewriteLine + "\n"
}

func ingressControllerClusterIP() string {
	for _, args := range [][]string{
		{"-n", "ingress-nginx", "get", "svc", "ingress-nginx-controller", "-o", "jsonpath={.spec.clusterIP}"},
		{"-n", "kube-system", "get", "svc", "traefik", "-o", "jsonpath={.spec.clusterIP}"},
	} {
		out, err := proc.RunW("kubectl", args...)
		if err == nil && strings.TrimSpace(out) != "" {
			return strings.TrimSpace(out)
		}
	}
	return ""
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
	base := harborAPIBase(host)

	if useHostHarborAPI() {
		By("checking Harbor chart repo via host API " + base)
		out, err := proc.RunW("curl", "-sk", "--max-time", "30",
			"-u", AdminUser+":"+password,
			base+"/api/v2.0/projects/"+ProjectName+"/repositories?page_size=100")
		Expect(err).NotTo(HaveOccurred(), out)
		Expect(out).To(ContainSubstring(repoPrefix))
		return
	}

	tmpDir := GinkgoT().TempDir()
	script := fmt.Sprintf(`set -e
body=$(curl -sk --max-time 30 -u '%[1]s:%[2]s' \
  '%[3]s/api/v2.0/projects/%[4]s/repositories?page_size=100' || true)
echo "$body"
echo "$body" | grep -q '%[5]s'
`, AdminUser, password, base, ProjectName, repoPrefix)

	runCurlScript(tmpDir, "harbor-expect-repo", script)
}

// useHostHarborAPI is true on k3d/GitHub runners where Harbor is reached via
// the published load-balancer port (EPINIO_PORT) from the host.
func useHostHarborAPI() bool {
	return os.Getenv("HARBOR_PATCH_INGRESS_DNS") == "1" || os.Getenv("EPINIO_PORT") != ""
}

// harborAPIBase returns https://host or https://host:port for Harbor HTTP API calls.
func harborAPIBase(host string) string {
	if p := os.Getenv("EPINIO_PORT"); p != "" {
		return "https://" + host + ":" + p
	}
	return "https://" + host
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

	base := harborAPIBase(host)
	if useHostHarborAPI() {
		By("creating Harbor project via host API " + base)
		createProjectFromHost(base, password)
		return
	}

	script := fmt.Sprintf(`set -e
for i in $(seq 1 60); do
  if curl -sk --max-time 30 -u '%[1]s:%[2]s' '%[3]s/api/v2.0/systeminfo' >/dev/null; then
    break
  fi
  sleep 5
done
code=$(curl -sk --max-time 30 -o /tmp/harbor-project.out -w '%%{http_code}' -u '%[1]s:%[2]s' \
  -H 'Content-Type: application/json' \
  -X POST '%[3]s/api/v2.0/projects' \
  -d '{"project_name":"%[4]s","public":false}' || true)
if [ "$code" != "201" ] && [ "$code" != "409" ]; then
  echo "create project failed: HTTP $code"
  cat /tmp/harbor-project.out || true
  exit 1
fi
`, AdminUser, password, base, ProjectName)

	runCurlScript(tmpDir, "harbor-create-project", script)
}

func createProjectFromHost(base, password string) {
	GinkgoHelper()

	Eventually(func() error {
		_, err := proc.RunW("curl", "-sk", "--max-time", "30",
			"-u", AdminUser+":"+password, base+"/api/v2.0/systeminfo")
		return err
	}, "5m", "5s").Should(Succeed())

	out, err := proc.RunW("curl", "-sk", "--max-time", "30",
		"-o", "/tmp/harbor-project.out", "-w", "%{http_code}",
		"-u", AdminUser+":"+password,
		"-H", "Content-Type: application/json",
		"-X", "POST", base+"/api/v2.0/projects",
		"-d", `{"project_name":"`+ProjectName+`","public":false}`)
	Expect(err).NotTo(HaveOccurred(), out)
	code := strings.TrimSpace(out)
	if code != "201" && code != "409" {
		body, _ := proc.RunW("cat", "/tmp/harbor-project.out")
		Fail(fmt.Sprintf("create project failed: HTTP %s\n%s", code, body))
	}
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
