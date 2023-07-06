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
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/dex"
	"github.com/pkg/errors"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
)

// LoginOIDC implements the "public client" flow of dex:
// https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
func (c *EpinioClient) LoginOIDC(ctx context.Context, address string, trustCA, prompt bool) error {
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
	// ask a token for the 'epinio-api' client
	oidcProvider.AddScopes("audience:server:client_id:epinio-api")

	token, err := c.generateToken(ctx, oidcProvider, prompt)
	if err != nil {
		return errors.Wrap(err, "error while asking for token")
	}

	updatedSettings.Token.AccessToken = token.AccessToken
	updatedSettings.Token.TokenType = token.TokenType
	updatedSettings.Token.Expiry = token.Expiry
	updatedSettings.Token.RefreshToken = token.RefreshToken

	// Clear any previous regular login settings
	updatedSettings.User = ""
	updatedSettings.Password = ""

	// get the custom headers of the original client
	customHeaders := c.API.Headers()

	// verify that settings are valid
	err = verifyCredentials(ctx, updatedSettings, customHeaders)
	if err != nil {
		return errors.Wrap(err, "error verifying credentials")
	}

	c.ui.Success().Msg("Login successful")

	err = updatedSettings.Save()
	return errors.Wrap(err, "error saving new settings")
}

// generateToken implements the Oauth2 flow to generate an auth token
func (c *EpinioClient) generateToken(ctx context.Context, oidcProvider *dex.OIDCProvider, prompt bool) (*oauth2.Token, error) {
	var authCode, codeVerifier string
	var err error

	if prompt {
		authCode, codeVerifier, err = c.getAuthCodeAndVerifierFromUser(oidcProvider)
	} else {
		authCode, codeVerifier, err = c.getAuthCodeAndVerifierWithServer(ctx, oidcProvider)
	}
	if err != nil {
		return nil, errors.Wrap(err, "error getting the auth code")
	}

	token, err := oidcProvider.ExchangeWithPKCE(ctx, authCode, codeVerifier)
	if err != nil {
		return nil, errors.Wrap(err, "exchanging with PKCE")
	}
	return token, nil
}

// getAuthCodeAndVerifierFromUser will wait for the user to input the auth code
func (c *EpinioClient) getAuthCodeAndVerifierFromUser(oidcProvider *dex.OIDCProvider) (string, string, error) {
	authCodeURL, codeVerifier := oidcProvider.AuthCodeURLWithPKCE()

	msg := c.ui.Normal().Compact()
	msg.Msg("\n" + authCodeURL)
	msg.Msg("\nOpen this URL in your browser and paste the authorization code:")

	var authCode string

	for authCode == "" {
		bytesCode, err := readUserInput()
		if err != nil {
			return "", "", errors.Wrap(err, "reading authCode user input")
		}
		authCode = strings.TrimSpace(string(bytesCode))
	}

	return authCode, codeVerifier, nil
}

// getAuthCodeAndVerifierWithServer will wait for the user to login and then will fetch automatically the auth code from the redirect URL
func (c *EpinioClient) getAuthCodeAndVerifierWithServer(ctx context.Context, oidcProvider *dex.OIDCProvider) (string, string, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", "", errors.Wrap(err, "creating listener")
	}
	oidcProvider.Config.Oauth2.RedirectURL = fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)

	authCodeURL, codeVerifier := oidcProvider.AuthCodeURLWithPKCE()

	msg := c.ui.Normal().Compact()
	msg.Msg("\n" + authCodeURL)
	msg.Msg("\nOpen this URL in your browser and follow the directions.")

	// if it fails to open the browser the user can still proceed manually
	_ = open.Run(authCodeURL)

	return startServerAndWaitForCode(ctx, listener), codeVerifier, nil
}

// startServerAndWaitForCode will start a local server to read automatically the auth code
func startServerAndWaitForCode(ctx context.Context, listener net.Listener) string {
	var authCode string

	srv := &http.Server{ReadHeaderTimeout: time.Second * 30}
	defer func() { _ = srv.Shutdown(ctx) }()

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

func trustCertInSettings(certsToTrust string) (*settings.Settings, error) {
	epinioSettings, err := settings.Load()
	if err != nil {
		return nil, errors.Wrap(err, "error loading the settings")
	}

	epinioSettings.Certs = certsToTrust

	return epinioSettings, nil
}
