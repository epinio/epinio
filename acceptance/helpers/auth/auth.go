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

package auth

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"

	"github.com/epinio/epinio/internal/dex"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

var ErrCodeNotFound = errors.New("code not found")

type DexClient struct {
	lastURL string
	dexURL  string

	Client *http.Client
}

func GetToken(domain, email, password string) (string, error) {
	dexURL := regexp.MustCompile(`epinio\.(.*)`).ReplaceAllString(domain, "auth.$1")

	dexClient, err := NewDexClient(dexURL)
	if err != nil {
		return "", errors.Wrap(err, "error creating http client")
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, dexClient.Client)

	oidcProvider, err := dex.NewOIDCProvider(ctx, dexURL, "epinio-cli")
	if err != nil {
		return "", errors.Wrap(err, "error creating OIDC provider")
	}
	// ask a token for the 'epinio-api' client
	oidcProvider.AddScopes("audience:server:client_id:epinio-api")

	// getting login URL (with redirect)
	authCodeURL, codeVerifier := oidcProvider.AuthCodeURLWithPKCE()

	// programmatic login
	authCode, err := dexClient.Login(authCodeURL, email, password)
	if err != nil {
		return "", errors.Wrapf(err, "error logging in with '%s'", email)
	}

	// exchange code for token
	token, err := oidcProvider.ExchangeWithPKCE(ctx, authCode, codeVerifier)
	if err != nil {
		return "", errors.Wrap(err, "error getting token with code")
	}

	return token.AccessToken, nil
}

// newClient creates an HttpClient with Session Storage and that will store the last redirect in the lastURL var
func NewDexClient(dexURL string) (*DexClient, error) {
	dexClient := &DexClient{
		dexURL: dexURL,
	}
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // nolint:gosec // tests using self signed certs

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "error creating CookieJar for HttpClient")
	}

	dexClient.Client = &http.Client{
		Transport: customTransport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			dexClient.lastURL = req.URL.RequestURI()
			return nil
		},
	}

	return dexClient, nil
}

func (c *DexClient) Login(loginURL, username, password string) (string, error) {
	_, err := c.Client.Get(loginURL)
	if err != nil {
		return "", errors.Wrap(err, "error getting redirect")
	}

	// do login
	loginURL = c.dexURL + c.lastURL
	res, err := c.Client.PostForm(loginURL, url.Values{
		"login":    []string{username},
		"password": []string{password},
	})
	if err != nil {
		return "", err
	}

	// get auth code from HTML
	html, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	// without adding a library to parse the HTML we are going to look for the first <input>,
	// that will contain the authCode value:
	//	 i.e.:	<input type="text" class="theme-form-input" value="c6ibus25hwnswxiv6z2wcip6p">
	for _, line := range strings.Split(string(html), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "input") {
			reg, err := regexp.Compile(`value="(.*)"`)
			if err != nil {
				return "", err
			}

			value := reg.FindStringSubmatch(line)
			if value == nil {
				return "", errors.Errorf("code not found in line [%s]", line)
			}
			return value[1], nil
		}
	}

	return "", ErrCodeNotFound
}
