package usercmd

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/settings"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

// Login will ask the user for a username and password, and then it will update the settings file accordingly
func (c *EpinioClient) Login(address string) error {
	log := c.Log.WithName("Login")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msgf("Login to your Epinio cluster [%s]", address)

	username, err := askUsername(c.ui)
	if err != nil {
		return errors.Wrap(err, "error while asking for username")
	}

	password, err := askPassword(c.ui)
	if err != nil {
		return errors.Wrap(err, "error while asking for password")
	}

	// check if the server has a trusted authority, or if we want to trust it anyway
	var serverCertificate string

	cert, err := checkCA(address)
	if err != nil {
		_, isUnknownAuthority := err.(x509.UnknownAuthorityError)
		// something bad happened while checking the certificates
		if !isUnknownAuthority {
			return errors.Wrap(err, "error while checking CA")
		}

		// certificate is signed by unknown authority
		// ask to the user if we want to trust it
		trustCA, err := askTrustCA(c.ui, cert)
		if err != nil {
			return errors.Wrap(err, "error while asking for trusting the CA")
		}

		// if yes then encode the certificate to PEM format and save it in the settings
		if trustCA {
			c.ui.Success().Msg("Trusting certificate...")
			pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
			serverCertificate = string(pemCert)
		}
	}

	updatedSettings, err := updateSettings(address, username, password, serverCertificate)
	if err != nil {
		return errors.Wrap(err, "error updating settings")
	}

	err = verifyCredentials(updatedSettings)
	if err != nil {
		return errors.Wrap(err, "error verifying credentials")
	}
	c.ui.Success().Msg("Login successful")

	err = updatedSettings.Save()
	return errors.Wrap(err, "error saving new settings")
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

		switch strings.ToLower(input) {
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

func readUserInput() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(s, "\n"), nil
}

func checkCA(address string) (*x509.Certificate, error) {
	parsedURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: true} // nolint:gosec // We need to check the validity
	conn, err := tls.Dial("tcp", parsedURL.Hostname()+":443", tlsConfig)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates to verify")
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

	return epinioSettings, nil
}

func verifyCredentials(epinioSettings *settings.Settings) error {
	if epinioSettings.Certs != "" {
		auth.ExtendLocalTrust(epinioSettings.Certs)
	}

	apiClient := epinioapi.New(epinioSettings.API, epinioSettings.WSS, epinioSettings.User, epinioSettings.Password)

	_, err := apiClient.Namespaces()
	return errors.Wrap(err, "error while connecting to the Epinio server")
}
