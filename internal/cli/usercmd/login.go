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

package usercmd

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/termui"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

// Login will ask the user for a username and password, and then it will update the settings file accordingly
func (c *EpinioClient) Login(ctx context.Context, username, password, address string, trustCA bool) error {
	var err error

	log := c.Log.WithName("Login")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msgf("Login to your Epinio cluster [%s]", address)

	if username == "" {
		username, err = askUsername(c.ui)
		if err != nil {
			return errors.Wrap(err, "error while asking for username")
		}
	}

	if password == "" {
		password, err = askPassword(c.ui)
		if err != nil {
			return errors.Wrap(err, "error while asking for password")
		}
	}

	// check if the server has a trusted authority, or if we want to trust it anyway
	serverCertificate, err := checkAndAskCA(c.ui, []string{address}, trustCA)
	if err != nil {
		return errors.Wrap(err, "error while checking CA")
	}

	// load settings and update them (in memory)
	updatedSettings, err := updateSettings(address, username, password, serverCertificate)
	if err != nil {
		return errors.Wrap(err, "error updating settings")
	}

	// get the custom headers of the original client
	customHeaders := c.API.Headers()

	// verify that settings are valid
	err = verifyCredentials(ctx, updatedSettings, customHeaders)
	if err != nil {
		return errors.Wrap(err, "error verifying credentials")
	}

	c.ui.Success().Msg("Login successful")

	err = updatedSettings.Save()
	if err != nil {
		return errors.Wrap(err, "error saving new settings")
	}

	// Verify that the targeted namespace (if any) exists in the targeted cluster.
	// If not report the issue, clear the information, and ask the user to chose a proper namespace.

	if updatedSettings.Namespace != "" {
		// Note: Create a new client for this. `c` cannot be assumed to be properly initialized,
		// as it was created before the login was performed and saved.

		client, err := New()
		if err != nil {
			return err
		}

		err = client.Init(ctx)
		if err != nil {
			return err
		}
		client.API.DisableVersionWarning()

		for k, values := range customHeaders {
			for _, v := range values {
				client.API.SetHeader(k, v)
			}
		}

		// we don't need anything, just checking if the namespace exist and we have permissions
		_, err = client.API.NamespaceShow(updatedSettings.Namespace)
		if err != nil {
			c.ui.Exclamation().Msgf("Current namespace '%s' not found in targeted cluster",
				updatedSettings.Namespace)

			updatedSettings.Namespace = ""

			err = updatedSettings.Save()
			if err != nil {
				return errors.Wrap(err, "error saving new settings")
			}

			c.ui.Exclamation().Msg("Cleared current namespace")
			c.ui.Exclamation().Msg("Please use `epinio target` to chose a new current namespace")
		}
	}

	return nil
}

func askUsername(ui *termui.UI) (string, error) {
	var username string
	var err error

	ui.Normal().Msg("")
	for username == "" {
		ui.Normal().Compact().KeepLine().Msg("Username: ")
		username, err = readUserInput()
		if err != nil {
			return "", err
		}
	}

	return username, nil
}

func askPassword(ui *termui.UI) (string, error) {
	var password string

	msg := ui.Normal().Compact()
	for password == "" {
		msg.KeepLine().Msg("Password: ")

		bytesPassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", err
		}

		password = strings.TrimSpace(string(bytesPassword))
		msg = ui.Normal()
	}
	ui.Normal().Msg("")

	return password, nil
}

func readUserInput() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// checkAndAskCA will check if the server has a trusted authority
// if the authority is unknown then we will prompt the user if he wants to trust it anyway
// and the func will return a list of PEM encoded certificates to trust (separated by new lines)
func checkAndAskCA(ui *termui.UI, addresses []string, trustCA bool) (string, error) {
	var builder strings.Builder

	// get all the certs to check
	certsToCheck := []*x509.Certificate{}
	for _, address := range addresses {
		cert, err := checkCA(address)
		if err != nil {
			// something bad happened while checking the certificates
			if cert == nil {
				return "", errors.Wrap(err, "error while checking CA")
			}
			// add the untrusted certificate to the list to check
			certsToCheck = append(certsToCheck, cert)
		} else {
			// and regularly trusted certs go directly into the result
			// This was missing in PR #1964, and demonstrated as bug with issue #2003
			pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
			builder.Write(pemCert)
		}
	}

	// in cert we trust!
	if trustCA {
		for i, cert := range certsToCheck {
			ui.Success().Msgf("Trusting certificate for address %s...", addresses[i])
			pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
			builder.Write(pemCert)
		}
		return builder.String(), nil
	}

	trustedIssuersMap := map[string]bool{}

	// let's prompt the user for every issuer
	for i, cert := range certsToCheck {
		var trustedCA, asked bool
		var err error

		trustedCA, asked = trustedIssuersMap[cert.Issuer.String()]

		if !asked {
			trustedCA, err = askTrustCA(ui, cert)
			if err != nil {
				return "", errors.Wrap(err, "error while asking to trust the CA")
			}
			trustedIssuersMap[cert.Issuer.String()] = trustedCA
		}

		// if the CA is trusted we can add the cert
		if trustedCA {
			ui.Success().Msgf("Trusting certificate for address %s...", addresses[i])
			pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
			builder.Write(pemCert)
		}
	}

	return builder.String(), nil
}

// checkCA will check if the address has a trusted certificate.
// If not trusted it returns the untrusted certificate and an error, otherwise if trusted then no error will be returned
func checkCA(address string) (*x509.Certificate, error) {
	parsedURL, err := url.Parse(address)
	if err != nil {
		return nil, errors.New("error parsing the address")
	}

	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: true} // nolint:gosec // We need to check the validity
	conn, err := tls.Dial("tcp", parsedURL.Hostname()+":"+port, tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error while dialing the server")
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, errors.New("no certificates to verify")
	}

	// check if at least one certificate in the chain is valid
	for _, cert := range certs {
		_, err = cert.Verify(x509.VerifyOptions{})
		// if it's valid we are good to go
		if err == nil {
			return cert, nil
		}
	}

	// if none of the certificates are valid, return the leaf cert with its error
	_, err = certs[0].Verify(x509.VerifyOptions{})
	return certs[0], err
}

func askTrustCA(ui *termui.UI, cert *x509.Certificate) (bool, error) {
	// prompt user if wants to accept untrusted certificate
	ui.Exclamation().Msg("Certificate signed by unknown authority")

	ui.Normal().
		WithTable("KEY", "VALUE").
		WithTableRow("Issuer Name", cert.Issuer.String()).
		WithTableRow("Common Name", cert.Issuer.CommonName).
		WithTableRow("Expiry", cert.NotAfter.Format("2006-January-02")).
		Msg("")

	ui.Normal().KeepLine().Msg("Do you want to trust it (y/n): ")

	for {
		input, err := readUserInput()
		if err != nil {
			return false, err
		}

		switch strings.TrimSpace(strings.ToLower(input)) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			ui.Normal().Compact().KeepLine().Msg("Please enter y or n: ")
			continue
		}
	}
}

func updateSettings(address, username, password, serverCertificate string) (*settings.Settings, error) {
	epinioSettings, err := settings.Load()
	if err != nil {
		return nil, errors.Wrap(err, "error loading the settings")
	}

	epinioSettings.API = address
	epinioSettings.WSS = strings.Replace(address, "https://", "wss://", 1)
	epinioSettings.User = username
	epinioSettings.Password = password
	epinioSettings.Certs = serverCertificate

	// Clear any previous oidc login settings
	epinioSettings.Token = settings.TokenSetting{}

	return epinioSettings, nil
}

func verifyCredentials(ctx context.Context, epinioSettings *settings.Settings, customHeaders http.Header) error {
	// Ensure that the settings have a location to get passed the client's check for it.
	// Here it is ok to not have an actual location and file, because we are logging in, and at
	// the end of the operation the relevant file will be created. Only having an API matters,
	// and we do.
	epinioSettings.Location = "fake"
	apiClient := epinioapi.New(ctx, epinioSettings)

	for k, values := range customHeaders {
		for _, v := range values {
			apiClient.SetHeader(k, v)
		}
	}

	_, err := apiClient.Me()
	// all good
	if err == nil {
		return nil
	}

	// check if the error was a NotFound, for backward compatibility.
	// Epinio versions up to v1.10.0 don't have the /me endpoint
	// TODO: remove this check in a couple of releases.
	epinioAPIError := &client.APIError{}
	if !errors.As(err, &epinioAPIError) ||
		epinioAPIError.StatusCode != http.StatusNotFound {
		return errors.Wrap(err, "error while connecting to the Epinio server")
	}

	// backward compatible check
	_, err = apiClient.Namespaces()
	if err != nil {
		return errors.Wrap(err, "error while connecting to the Epinio server")
	}
	return nil
}
