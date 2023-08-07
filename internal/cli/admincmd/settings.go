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

// Package admincmd provides the commands of the admin CLI, which deals with
// installing and configurations
package admincmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/settings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const (
	epinioAPIProtocol = "https"
	epinioWSProtocol  = "wss"
	DefaultNamespace  = "workspace"
)

// Admin provides functionality for administering Epinio installations on
// Kubernetes
type Admin struct {
	Settings *settings.Settings
	Log      logr.Logger
	ui       *termui.UI
}

func New() (*Admin, error) {
	settingsSettings, err := settings.Load()
	if err != nil {
		return nil, err
	}

	uiUI := termui.NewUI()

	logger := tracelog.NewLogger().WithName("EpinioSettings").V(3)

	return &Admin{
		ui:       uiUI,
		Settings: settingsSettings,
		Log:      logger,
	}, nil
}

// SettingsUpdateCA updates the CA credentials stored in the settings from the
// currently targeted kube cluster. It does not use the API server.
func (a *Admin) SettingsUpdateCA(ctx context.Context) error {
	log := a.Log.WithName("SettingsUpdateCA")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	a.ui.Note().
		WithStringValue("Settings", helpers.AbsPath(a.Settings.Location)).
		Msg("Updating CA in the stored credentials from the current cluster")

	if a.Settings.Location == "" {
		return errors.New("settings file not found")
	}

	details.Info("retrieving server locations")

	api := a.Settings.API
	wss := a.Settings.WSS

	details.Info("retrieved server locations", "api", api, "wss", wss)
	details.Info("retrieving certs")

	certs, err := encodeCertificate(api)
	if err != nil {
		a.ui.Exclamation().Msg(err.Error())
		return nil
	}

	details.Info("retrieved certs", "certs", certs)

	a.Settings.Certs = certs

	details.Info("saving",
		"user", a.Settings.User,
		"pass", a.Settings.Password,
		"access_token", a.Settings.Token.AccessToken,
		"api", a.Settings.API,
		"wss", a.Settings.WSS,
		"cert", a.Settings.Certs)

	err = a.Settings.Save()
	if err != nil {
		a.ui.Exclamation().Msg(errors.Wrap(err, "failed to save configuration").Error())
		return nil
	}

	details.Info("saved")

	a.ui.Success().Msg("Ok")
	return nil
}

func encodeCertificate(address string) (string, error) {
	var builder strings.Builder

	cert, err := checkCA(address)
	if err != nil {
		// something bad happened while checking the certificates
		if cert == nil {
			return "", errors.Wrap(err, "error while checking CA")
		}
		// add the untrusted certificate
		pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		builder.Write(pemCert)
	} else {
		// and regularly trusted certs go directly into the result
		// This was missing in PR #1964, and demonstrated as bug with issue #2003
		pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		builder.Write(pemCert)
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
