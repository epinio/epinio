package usercmd

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/dex"
	epinioapi "github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/pkg/errors"
	"github.com/skratchdot/open-golang/open"
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
	dexURL := regexp.MustCompile(`epinio\.(.*)`).ReplaceAllString(address, "auth.$1")

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

	oidcProvider, err := dex.NewOIDCProvider(ctx, dexURL, "epinio-cli")
	if err != nil {
		return errors.Wrap(err, "constructing dexProviderConfig")
	}

	token, err := generateToken(ctx, c.ui, oidcProvider)
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
func generateToken(ctx context.Context, ui *termui.UI, oidcProvider *dex.OIDCProvider) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, errors.Wrap(err, "creating listener")
	}
	oidcProvider.Config.RedirectURL = fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)

	authCodeURL, codeVerifier := oidcProvider.AuthCodeURLWithPKCE()

	msg := ui.Normal().Compact()
	msg.Msg("\n" + authCodeURL)
	msg.Msg("\nOpen this URL in your browser and follow the directions.")

	// if it fails to open the browser the user can still proceed manually
	_ = open.Run(authCodeURL)

	authCode := getAuthCodeWithServer(listener)

	// for authCode == "" {
	// 	bytesCode, err := readUserInput()
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	authCode = strings.TrimSpace(string(bytesCode))
	// }

	token, err := oidcProvider.ExchangeWithPKCE(ctx, authCode, codeVerifier)
	if err != nil {
		return nil, errors.Wrap(err, "exchanging with PKCE")
	}
	return token, nil
}

func getAuthCodeWithServer(listener net.Listener) string {
	var authCode string

	srv := &http.Server{}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	wg := &sync.WaitGroup{}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		authCode = r.URL.Query().Get("code")
		fmt.Fprintf(w, "Login successful! You can close this window.")
		wg.Done()
	})

	wg.Add(1)
	go func() { _ = srv.Serve(listener) }()
	wg.Wait()

	return authCode
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
		}
		certsToCheck = append(certsToCheck, cert)
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
