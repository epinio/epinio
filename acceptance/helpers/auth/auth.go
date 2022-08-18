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

var (
	lastURL string
	dexURL  string
)

func GetToken(domain, email string) (string, error) {
	dexURL = regexp.MustCompile(`epinio\.(.*)`).ReplaceAllString(domain, "auth.$1")
	client, err := newClient(&lastURL)
	if err != nil {
		return "", errors.Wrap(err, "error creating http client")
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)

	oidcProvider, err := dex.NewOIDCProvider(ctx, dexURL, "epinio-cli")
	if err != nil {
		return "", errors.Wrap(err, "error creating OIDC provider")
	}

	// getting login URL (with redirect)
	authCodeURL, codeVerifier := oidcProvider.AuthCodeURLWithPKCE()
	_, _ = client.Get(authCodeURL)

	// programmatic login
	authCode, err := login(client, email, "password")
	if err != nil {
		return "", errors.Wrap(err, "error logging in with 'admin@epinio.io'")
	}

	// exchange code for token
	token, err := oidcProvider.ExchangeWithPKCE(ctx, authCode, codeVerifier)
	if err != nil {
		return "", errors.Wrap(err, "error getting token with code")
	}

	return token.AccessToken, nil
}

// newClient creates an HttpClient with Session Storage and that will store the last redirect in the lastURL var
func newClient(lastURL *string) (*http.Client, error) {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // nolint:gosec // tests using self signed certs

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "error creating CookieJar for HttpClient")
	}

	return &http.Client{
		Transport: customTransport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			*lastURL = req.URL.RequestURI()
			return nil
		},
	}, nil
}

func login(client *http.Client, username, password string) (string, error) {
	// do login
	loginURL := dexURL + lastURL
	_, err := client.PostForm(loginURL, url.Values{
		"login":    []string{username},
		"password": []string{password},
	})
	if err != nil {
		return "", err
	}

	// approve request
	approvalURL := dexURL + lastURL
	res, err := client.PostForm(approvalURL, url.Values{"approval": []string{"approve"}})
	if err != nil {
		return "", err
	}

	// get auth code from HTML
	html, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	// this is a very ugly/hacky way, but it's just for testing
	for _, line := range strings.Split(string(html), "\n") {
		if strings.Contains(line, "value") {
			return strings.Split(line, `"`)[5], nil
		}
	}

	return "", nil
}
