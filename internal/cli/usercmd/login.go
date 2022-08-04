package usercmd

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/dex"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// Login implements the "public client" flow of dex:
// https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
func (c *EpinioClient) Login(ctx context.Context, address string, trustCA bool) error {
	var err error

	log := c.Log.WithName("Login")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msgf("Login to your Epinio cluster [%s]", address)

	// Deduct the dex URL from the epinio one
	dexURL := regexp.MustCompile(`epinio\.(.*)`).ReplaceAllString(address, "dex.$1")

	// check if the server has a trusted authority, or if we want to trust it anyway
	certsToTrust, err := checkAndAskCA(c.ui, []string{address, dexURL}, trustCA)
	if err != nil {
		return errors.Wrap(err, "error while checking CA")
	}

	// load settings and update them (in memory)
	updatedSettings, err := trustCertInSettings(certsToTrust)
	if err != nil {
		return errors.Wrap(err, "error updating settings")
	}

	updatedSettings.API = address
	updatedSettings.WSS = strings.Replace(address, "https://", "wss://", 1)

	// Trust the cert to allow the client to talk to dex
	auth.ExtendLocalTrust(updatedSettings.Certs)

	token, err := generateToken(ctx, c.ui, dexURL)
	if err != nil {
		return errors.Wrap(err, "error while asking for token")
	}

	// TODO store also the RefreshToken and implement refresh flow
	updatedSettings.AccessToken = token.AccessToken

	// verify that settings are valid
	err = verifyCredentials(updatedSettings)
	if err != nil {
		return errors.Wrap(err, "error verifying credentials")
	}

	c.ui.Success().Msg("Login successful")

	err = updatedSettings.Save()
	return errors.Wrap(err, "error saving new settings")
}

// generateToken implements the Oauth2 flow to generate an auth token
func generateToken(ctx context.Context, ui *termui.UI, dexURL string) (*oauth2.Token, error) {
	// TODO: Hardcoded credentials?
	oauth2Config, err := dex.Oauth2Config(ctx, dexURL, "epinio-cli", "cli-app-secret")
	if err != nil {
		return nil, errors.Wrap(err, "constructing oauth2Config")
	}

	authCodeURL := oauth2Config.AuthCodeURL(dex.AppState)

	msg := ui.Normal().Compact()
	msg.Msg(authCodeURL)
	msg.Msg("Open this URL in your browser and follow the directions. Paste the result here: ")

	var authCode string
	for authCode == "" {
		bytesCode, err := readUserInput()
		if err != nil {
			return nil, err
		}
		authCode = strings.TrimSpace(string(bytesCode))
	}

	token, err := oauth2Config.Exchange(ctx, authCode)
	if err != nil {
		return nil, errors.Wrap(err, "exchanging code for token")
	}
	return token, nil
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
	var certsToTrust []string
	var trustedIssuers []pkix.Name

	for _, address := range addresses {
		promptTrust := false
		cert, err := checkCA(address)
		if err != nil {
			// something bad happened while checking the certificates
			if cert == nil {
				return "", errors.Wrap(err, "error while checking CA")
			}

			alreadyTrustedIssuer := func(trustedIssuers []pkix.Name, issuer pkix.Name) bool {
				for _, i := range trustedIssuers {
					if i.String() == issuer.String() {
						return true
					}
				}
				return false
			}(trustedIssuers, cert.Issuer)

			// certificate is signed by unknown authority
			// ask to the user if we want to trust it
			if !trustCA && !alreadyTrustedIssuer {
				promptTrust, err = askTrustCA(ui, cert)
				if err != nil {
					return "", errors.Wrap(err, "error while asking for trusting the CA")
				}
				if promptTrust {
					trustedIssuers = append(trustedIssuers, cert.Issuer)
				}
			}

			// if yes then encode the certificate to PEM format and save it in the settings
			if trustCA || promptTrust || alreadyTrustedIssuer {
				ui.Success().Msgf("Trusting certificate for address %s...", address)
				pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
				certsToTrust = append(certsToTrust, string(pemCert))
			}
		}
	}

	return strings.Join(certsToTrust, "\n"), nil
}

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

func trustCertInSettings(certsToTrust string) (*settings.Settings, error) {
	epinioSettings, err := settings.Load()
	if err != nil {
		return nil, errors.Wrap(err, "error loading the settings")
	}

	epinioSettings.Certs = certsToTrust

	return epinioSettings, nil
}

func verifyCredentials(epinioSettings *settings.Settings) error {
	apiClient := epinioapi.New(epinioSettings)
	_, err := apiClient.Namespaces()
	return errors.Wrap(err, "error while connecting to the Epinio server")
}
