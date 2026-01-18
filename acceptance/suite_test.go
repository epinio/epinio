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
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/auth"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/internal/cli/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "Acceptance Suite")
}

var (
	// Labels for test sections.
	LService       = Label("service")
	LAppchart      = Label("appchart")
	LApplication   = Label("application")
	LConfiguration = Label("configuration")
	LNamespace     = Label("namespace")
	LGitconfig     = Label("gitconfig")
	LMisc          = Label("misc")

	// Test configuration and state
	nodeSuffix, nodeTmpDir  string
	serverURL, websocketURL string

	env testenv.EpinioEnv
	r   *rand.Rand
)

// BeforeSuiteMessage is a serializable struct that can be passed through the SynchronizedBeforeSuite
type BeforeSuiteMessage struct {
	AdminToken string `json:"admin_token"`
	UserToken  string `json:"user_token"`
}

var _ = SynchronizedBeforeSuite(func() []byte {
	testenv.SetRoot("..")
	epinioBinaryPath := testenv.EpinioBinaryPath()
	if _, err := os.Stat(epinioBinaryPath); err != nil {
		if os.IsNotExist(err) {
			testenv.BuildEpinio()
		} else {
			Expect(err).NotTo(HaveOccurred())
		}
	}

	derivedServerURL, derivedWebsocketURL, err := deriveServerURLs()
	Expect(err).NotTo(HaveOccurred())

	globalSettings, err := ensureSettingsFile(derivedServerURL, derivedWebsocketURL)
	Expect(err).NotTo(HaveOccurred())

	// Try to get tokens, but don't fail the entire suite if it fails
	// This allows tests to run and handle auth failures individually
	adminToken, err := getTokenWithRetry(globalSettings.API, "admin@epinio.io", "password")
	if err != nil {
		fmt.Printf("WARNING: Failed to get admin token: %v\n", err)
		fmt.Println("Tests will continue but may fail if authentication is required")
		adminToken = "" // Set empty token, let tests handle auth failures
	}
	
	userToken, err := getTokenWithRetry(globalSettings.API, "epinio@epinio.io", "password")
	if err != nil {
		fmt.Printf("WARNING: Failed to get user token: %v\n", err)
		fmt.Println("Tests will continue but may fail if authentication is required")
		userToken = "" // Set empty token, let tests handle auth failures
	}

	msg, err := json.Marshal(BeforeSuiteMessage{
		AdminToken: adminToken,
		UserToken:  userToken,
	})
	Expect(err).NotTo(HaveOccurred())

	return msg
}, func(msg []byte) {
	r = rand.New(rand.NewSource(time.Now().UnixNano()))

	var message BeforeSuiteMessage
	err := json.Unmarshal(msg, &message)
	Expect(err).NotTo(HaveOccurred())

	fmt.Printf("Running tests on node %d\n", GinkgoParallelProcess())

	testenv.SetRoot("..")
	testenv.SetupEnv()

	nodeSuffix = fmt.Sprintf("%d", GinkgoParallelProcess())
	nodeTmpDir, err := os.MkdirTemp("", "epinio-"+nodeSuffix)
	Expect(err).NotTo(HaveOccurred())

	out, err := testenv.CopyEpinioSettings(nodeTmpDir)
	Expect(err).ToNot(HaveOccurred(), out)
	os.Setenv("EPINIO_SETTINGS", nodeTmpDir+"/epinio.yaml")

	theSettings, err := settings.LoadFrom(nodeTmpDir + "/epinio.yaml")
	Expect(err).NotTo(HaveOccurred())

	env = testenv.New(nodeTmpDir, testenv.Root(), theSettings.User, theSettings.Password, message.AdminToken, message.UserToken)

	serverURL, websocketURL, err = deriveServerURLs()
	Expect(err).ToNot(HaveOccurred())

	// Update the settings file with the correct API URL
	// This ensures ShowApp and other functions that use GetSettings() work correctly
	theSettings.API = serverURL
	theSettings.WSS = websocketURL
	err = theSettings.Save()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if !testenv.SkipCleanup() {
		fmt.Printf("Deleting tmpdir on node %d\n", GinkgoParallelProcess())
		testenv.DeleteTmpDir(nodeTmpDir)
	}
})

var _ = AfterEach(func() {
	testenv.AfterEachSleep()
})

func FailWithReport(message string, callerSkip ...int) {
	// NOTE: Use something like the following if you need to debug failed tests
	// fmt.Println("\nA test failed. You may find the following information useful for debugging:")
	// fmt.Println("The cluster pods: ")
	// out, err := proc.Kubectl("get pods --all-namespaces")
	// if err != nil {
	// 	fmt.Print(err.Error())
	// } else {
	// 	fmt.Print(out)
	// }

	// Ensures the correct line numbers are reported
	Fail(message, callerSkip[0]+1)
}

// getPortSuffixFromServerURL extracts the port suffix (with colon prefix) from serverURL.
// Returns the port with a colon prefix, e.g., ":8443" from "https://example.com:8443".
// Returns empty string for default HTTPS port (443) or if no port is specified.
// Falls back to ":8443" if parsing fails.
func getPortSuffixFromServerURL() string {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		// If parsing fails, return default port
		return ":8443"
	}

	port := parsed.Port()
	if port == "" {
		// No port specified - for HTTPS, default is 443, return empty string
		// (routes will work without explicit port for standard HTTPS)
		return ""
	}

	// If port is 443, return empty string (standard HTTPS, no need to append)
	if port == "443" {
		return ""
	}

	return ":" + port
}

func waitForDexService(dexURL string, timeout time.Duration) error {
	// Check if we should skip the readiness check
	if os.Getenv("EPINIO_SKIP_DEX_CHECK") == "true" {
		fmt.Println("Skipping Dex readiness check (EPINIO_SKIP_DEX_CHECK=true)")
		return nil
	}
	
	start := time.Now()
	interval := 5 * time.Second
	
	// First, check if Dex pod is ready
	fmt.Println("Waiting for Dex service to be ready...")
	for {
		// Check if Dex pod is running and ready
		// Try multiple label selectors in case the label format differs
		labels := []string{
			"app.kubernetes.io/name=dex",
			"app=dex",
			"component=dex",
		}
		
		podReady := false
		for _, label := range labels {
			out, err := proc.Kubectl("get", "pods", "-n", "epinio", "-l", label, "-o", "jsonpath={.items[0].status.phase}")
			if err == nil && strings.TrimSpace(out) == "Running" {
				// Also check if it's ready (not just running)
				ready, _ := proc.Kubectl("get", "pods", "-n", "epinio", "-l", label, "-o", "jsonpath={.items[0].status.conditions[?(@.type=='Ready')].status}")
				if strings.TrimSpace(ready) == "True" {
					fmt.Println("Dex pod is ready")
					podReady = true
					break
				}
			}
		}
		
		if podReady {
			break
		}
		
		if time.Since(start) >= timeout {
			// Get more diagnostic info - try all label selectors
			var allPods strings.Builder
			for _, label := range labels {
				pods, _ := proc.Kubectl("get", "pods", "-n", "epinio", "-l", label)
				if pods != "" {
					allPods.WriteString(fmt.Sprintf("Pods with label %s:\n%s\n", label, pods))
				}
			}
			// Also get all pods in epinio namespace for debugging
			allEpPods, _ := proc.Kubectl("get", "pods", "-n", "epinio")
			return fmt.Errorf("Dex service not ready after %v.\n%s\nAll pods in epinio namespace:\n%s", timeout, allPods.String(), allEpPods)
		}
		
		time.Sleep(interval)
	}
	
	// Now try to connect to the service - try both the provided URL and port 443
	fmt.Println("Verifying Dex service connectivity...")
	dexClient, err := auth.NewDexClient(dexURL)
	if err != nil {
		return fmt.Errorf("failed to create Dex client: %w", err)
	}
	
	// Try the provided URL first, then try with port 443 if it fails
	urlsToTry := []string{dexURL + "/.well-known/openid-configuration"}
	
	// If the URL has a non-standard port (like 8443), also try 443
	parsedURL, err := url.Parse(dexURL)
	if err == nil && parsedURL.Port() != "" && parsedURL.Port() != "443" {
		// Try with port 443
		url443 := fmt.Sprintf("%s://%s:443/.well-known/openid-configuration", parsedURL.Scheme, parsedURL.Hostname())
		urlsToTry = append(urlsToTry, url443)
		// Also try without explicit port (defaults to 443 for HTTPS)
		urlDefault := fmt.Sprintf("%s://%s/.well-known/openid-configuration", parsedURL.Scheme, parsedURL.Hostname())
		urlsToTry = append(urlsToTry, urlDefault)
	}
	
	connectTimeout := 30 * time.Second
	connectStart := time.Now()
	
	for {
		var lastErr error
		for _, healthURL := range urlsToTry {
			req, err := http.NewRequest("GET", healthURL, nil)
			if err != nil {
				lastErr = fmt.Errorf("failed to create request: %w", err)
				continue
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			req = req.WithContext(ctx)
			
			resp, err := dexClient.Client.Do(req)
			cancel()
			
			if err == nil && resp != nil {
				if resp.StatusCode == 200 {
					resp.Body.Close()
					fmt.Printf("Dex service is accessible at %s\n", healthURL)
					return nil
				}
				resp.Body.Close()
				lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			} else {
				lastErr = err
			}
		}
		
		if time.Since(connectStart) >= connectTimeout {
			return fmt.Errorf("Dex service not accessible after %v. Tried: %v. Last error: %w", connectTimeout, urlsToTry, lastErr)
		}
		
		time.Sleep(2 * time.Second)
	}
}

func getTokenWithRetry(apiURL, email, password string) (string, error) {
	// Derive Dex URL from API URL
	dexURL := regexp.MustCompile(`epinio\.(.*)`).ReplaceAllString(apiURL, "auth.$1")
	
	// Wait for Dex service to be ready before attempting token retrieval
	oidcRetryTimeout, _ := oidcRetryConfig()
	if err := waitForDexService(dexURL, oidcRetryTimeout); err != nil {
		return "", fmt.Errorf("Dex service not ready: %w", err)
	}
	
	oidcRetryTimeout, oidcRetryInterval := oidcRetryConfig()
	start := time.Now()
	var token string
	var err error
	for {
		token, err = auth.GetToken(apiURL, email, password)
		if err == nil || !isTransientOIDCError(err) {
			return token, err
		}
		if time.Since(start) >= oidcRetryTimeout {
			return token, err
		}
		time.Sleep(oidcRetryInterval)
	}
}

func oidcRetryConfig() (time.Duration, time.Duration) {
	timeout := envDuration("EPINIO_OIDC_RETRY_TIMEOUT", 5*time.Minute)
	interval := envDuration("EPINIO_OIDC_RETRY_INTERVAL", 10*time.Second)
	return timeout, interval
}

func envDuration(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	// Allow plain numbers to mean seconds.
	if secs, err := time.ParseDuration(raw + "s"); err == nil {
		return secs
	}
	return fallback
}

func isTransientOIDCError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "tls handshake timeout") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "timeout")
}

func deriveServerURLs() (string, string, error) {
	out, err := proc.Run(testenv.Root(), false, "kubectl", "get", "ingress",
		"--namespace", "epinio", "epinio",
		"-o", "jsonpath={.spec.rules[0].host}")
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch ingress host: %w: %s", err, out)
	}
	host := strings.TrimSpace(out)
	if host == "" {
		return "", "", fmt.Errorf("ingress host is empty")
	}

	port := os.Getenv("EPINIO_PORT")
	if port == "" {
		port = "8443"
	}

	if port == "443" {
		return "https://" + host, "wss://" + host, nil
	}

	return "https://" + host + ":" + port, "wss://" + host + ":" + port, nil
}

func ensureSettingsFile(serverURL, websocketURL string) (*settings.Settings, error) {
	settingsPath := testenv.EpinioYAML()
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
			return nil, fmt.Errorf("failed to create settings dir: %w", err)
		}
		if err := os.WriteFile(settingsPath, []byte(""), 0600); err != nil {
			return nil, fmt.Errorf("failed to create settings file: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat settings file: %w", err)
	}

	cfg, err := settings.LoadFrom(settingsPath)
	if err != nil {
		return nil, err
	}

	changed := false
	if cfg.API == "" {
		cfg.API = serverURL
		changed = true
	}
	if cfg.WSS == "" {
		cfg.WSS = websocketURL
		changed = true
	}
	if cfg.User == "" {
		cfg.User = "epinio"
		changed = true
	}
	if cfg.Password == "" {
		cfg.Password = "password"
		changed = true
	}
	if cfg.Certs == "" {
		certs, err := fetchServerCertificate(serverURL)
		if err != nil {
			return nil, err
		}
		cfg.Certs = certs
		changed = true
	}

	if changed {
		if err := cfg.Save(); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func fetchServerCertificate(address string) (string, error) {
	parsedURL, err := url.Parse(address)
	if err != nil {
		return "", fmt.Errorf("failed to parse server address: %w", err)
	}

	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: true} // nolint:gosec // used in acceptance tests
	conn, err := tls.Dial("tcp", parsedURL.Hostname()+":"+port, tlsConfig)
	if err != nil {
		return "", fmt.Errorf("failed to connect to server for certificates: %w", err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("no certificates returned by server")
	}

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certs[0].Raw})
	return string(pemCert), nil
}

