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

package acceptance_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/auth"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func ExpectGoodUserLogin(tmpSettingsPath, password, serverURL string) string {
	By("Regular login")
	loginURL := testenv.AppRouteWithPort(serverURL)
	waitForServerReady(loginURL)

	var out string
	Eventually(func() bool {
		var err error
		out, err = env.Epinio("", "login", "-u", "epinio", "-p", password,
			"--trust-ca", "--settings-file", tmpSettingsPath, loginURL)
		if err == nil {
			return true
		}
		if isTransientConnectFailure(out) {
			fmt.Fprintf(GinkgoWriter, "[ExpectGoodUserLogin] transient login failure at %s: %v\n%s\n", loginURL, err, out)
			return false
		}
		Fail(fmt.Sprintf("login failed unexpectedly at %s: %v\n%s", loginURL, err, out))
		return false
	}, "2m", "3s").Should(BeTrue(), out)

	Expect(out).To(ContainSubstring(`Login to your Epinio cluster`))
	Expect(out).To(ContainSubstring(`Trusting certificate`))
	Expect(out).To(ContainSubstring(`Login successful`))
	By("Regular login done")

	return out
}

func ExpectGoodTokenLogin(tmpSettingsPath, serverURL string) {
	By("OIDC login")
	loginURL := testenv.AppRouteWithPort(serverURL)
	waitForServerReady(loginURL)

	out := &bytes.Buffer{}
	cmd := env.EpinioCmd("login", "--prompt", "--oidc",
		"--trust-ca", "--settings-file", tmpSettingsPath, loginURL)
	cmd.Stdout = out
	cmd.Stderr = out

	stdinPipe, err := cmd.StdinPipe()
	Expect(err).ToNot(HaveOccurred())

	iscomplete := make(chan error, 1)

	// run the epinio login and wait for the input of the authCode
	go func() {
		By("Background: login")
		defer GinkgoRecover()

		err = cmd.Run()
		By("Background: signal run completion")
		iscomplete <- err
	}()

	// Read output until prompt appears, but fail fast if login exits early.
	By("Waiting for auth code query")
	var fullOutput string
	Eventually(func() string {
		fullOutput = out.String()
		lower := strings.ToLower(fullOutput)
		if strings.Contains(lower, "authorization code") {
			return "prompted"
		}

		select {
		case runErr := <-iscomplete:
			if runErr != nil {
				fmt.Fprintf(GinkgoWriter, "[ExpectGoodTokenLogin] login command exited early at %s: %v\n%s\n", loginURL, runErr, fullOutput)
			}
			Expect(runErr).ToNot(HaveOccurred(), fullOutput)
			return "finished"
		default:
			return "waiting"
		}
	}, "2m", "200ms").Should(Equal("prompted"), fullOutput)

	Expect(fullOutput).To(ContainSubstring(`Login to your Epinio cluster`))
	Expect(fullOutput).To(ContainSubstring(`Trusting certificate`))

	lines := strings.Split(fullOutput, "\n")

	var authURL string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://auth") {
			authURL = line
			break
		}
	}
	Expect(authURL).ToNot(BeEmpty())

	// authenticate with Dex, get the authCode and submit the input to the waiting command
	u, err := url.Parse(authURL)
	Expect(err).ToNot(HaveOccurred())
	loginClient, err := auth.NewDexClient(fmt.Sprintf("%s://%s", u.Scheme, u.Host))
	Expect(err).ToNot(HaveOccurred())

	authCode, err := loginClient.Login(authURL, "admin@epinio.io", "password")
	Expect(err).ToNot(HaveOccurred())

	By("Piping auth data into command")
	_, err = fmt.Fprintln(stdinPipe, authCode)
	Expect(err).ToNot(HaveOccurred())

	By("Waiting for login completion")
	select {
	case err = <-iscomplete:
	case <-time.After(2 * time.Minute):
		Fail(fmt.Sprintf("timed out waiting for login completion. Output so far:\n%s", out.String()))
	}
	Expect(err).ToNot(HaveOccurred(), out.String())

	// after the command terminates check that the login was successful
	Expect(out.String()).To(ContainSubstring(`Login successful`))
	By("OIDC login done")
}

func waitForServerReady(serverURL string) {
	readyURL := strings.TrimSuffix(serverURL, "/") + "/ready"
	By("Waiting for API readiness: " + readyURL)
	Eventually(func() bool {
		resp, err := env.Curl("GET", readyURL, nil)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "[waitForServerReady] curl error %s: %v\n", readyURL, err)
			return false
		}
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		if resp == nil || resp.StatusCode != 200 {
			if resp != nil {
				fmt.Fprintf(GinkgoWriter, "[waitForServerReady] non-200 %s: %d\n", readyURL, resp.StatusCode)
			}
			return false
		}
		return true
	}, "2m", "3s").Should(BeTrue(), "API never became ready at %s", readyURL)
}

const s3HelperPod = "s3cli"

// createS3HelperPod starts a long-lived minio/mc pod used to manipulate S3
// blobs in acceptance tests. Safe to call multiple times — exits early if the
// pod already exists and is ready.
func createS3HelperPod() {
	out, err := proc.Kubectl("get", "pod", "-o", "name", s3HelperPod)
	if err != nil {
		Expect(out).To(MatchRegexp("not found"))
	}
	if strings.TrimSpace(out) == "pod/"+s3HelperPod {
		return
	}

	out, err = proc.Kubectl("get", "secret",
		"-n", "epinio",
		"seaweedfs-creds", "-o", "jsonpath={.data.accesskey}")
	Expect(err).ToNot(HaveOccurred(), out)
	accessKey, err := base64.StdEncoding.DecodeString(out)
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = proc.Kubectl("get", "secret",
		"-n", "epinio",
		"seaweedfs-creds", "-o", "jsonpath={.data.secretkey}")
	Expect(err).ToNot(HaveOccurred(), out)
	secretKey, err := base64.StdEncoding.DecodeString(out)
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = proc.Kubectl("run", s3HelperPod,
		"--image=minio/mc:RELEASE.2022-03-13T22-34-00Z",
		"--command", "--", "/bin/bash", "-c",
		"trap : TERM INT; sleep infinity & wait")
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = proc.Kubectl("wait", "--for=condition=ready", "pod", s3HelperPod)
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = proc.Kubectl("exec", s3HelperPod, "--",
		"mc", "--insecure", "alias", "set", "s3",
		"http://seaweedfs-s3.epinio.svc.cluster.local:8333",
		string(accessKey), string(secretKey))
	Expect(err).ToNot(HaveOccurred(), out)
}

// deleteS3Blob removes a single blob from the epinio S3 bucket. Requires
// createS3HelperPod to have been called first.
func deleteS3Blob(blobUID string) {
	out, err := proc.Kubectl("exec", s3HelperPod, "--",
		"mc", "--insecure", "rm", "s3/epinio/"+blobUID)
	Expect(err).ToNot(HaveOccurred(), out)
}

func isTransientConnectFailure(out string) bool {
	l := strings.ToLower(out)
	return strings.Contains(l, "connect: connection refused") ||
		strings.Contains(l, "error while checking ca") ||
		strings.Contains(l, "dial tcp")
}

func ExpectEmptySettings(tmpSettingsPath string) {
	By("Check for empty settings")
	// check that the settings are empty
	settings, err := env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
	Expect(err).ToNot(HaveOccurred(), settings)
	Expect(settings).To(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("API User Name", ""),
			WithRow("API Password", ""),
			WithRow("API Token", ""),
			WithRow("Certificates", "None defined"),
		),
	)
}

func ExpectNamespace(tmpSettingsPath, namespace string) {
	By("Check for namespace `" + namespace + "`")
	// check that the namespace is not set
	settings, err := env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
	Expect(err).ToNot(HaveOccurred(), settings)
	Expect(settings).To(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("Current Namespace", namespace),
		),
	)
}

func ExpectUserPasswordSettings(tmpSettingsPath string) {
	By("Check for user/pass settings")
	// check that the settings contain user/assword authentication
	settings, err := env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
	Expect(err).ToNot(HaveOccurred(), settings)
	Expect(settings).To(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("API User Name", "epinio"),
			WithRow("API Password", "[*]+"),
			WithRow("API Token", ""),
			WithRow("Certificates", "Present"),
		),
	)
}

func ExpectTokenSettings(tmpSettingsPath string) {
	By("Check for token settings")
	// check that the settings contain user/assword authentication
	settings, err := env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
	Expect(err).ToNot(HaveOccurred(), settings)
	Expect(settings).To(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("API User Name", ""),
			WithRow("API Password", ""),
			WithRow("API Token", "[*]+"),
			WithRow("Certificates", "Present"),
		),
	)
}
