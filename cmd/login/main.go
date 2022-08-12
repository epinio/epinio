package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/epinio/epinio/internal/dex"
	"golang.org/x/oauth2"
)

var (
	lastURL string
	dexURL  = "https://auth.172.21.0.4.omg.howdoi.website"
)

func main() {
	client := newClient(&lastURL)
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)

	oidcProvider := checkErr(dex.NewOIDCProvider(
		ctx,
		dexURL,
		"epinio-cli",
	))

	authCodeURL, codeVerifier := oidcProvider.AuthCodeURLWithPKCE()
	_, _ = client.Get(authCodeURL)

	authCode := login(client, "", "")

	token := checkErr(oidcProvider.ExchangeWithPKCE(ctx, authCode, codeVerifier))

	b := checkErr(json.Marshal(token))
	fmt.Println(string(b))
}

func newClient(lastURL *string) *http.Client {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &http.Client{
		Transport: customTransport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			*lastURL = req.URL.RequestURI()
			return nil
		},
	}
}

func login(client *http.Client, username, password string) string {
	// do login
	loginURL := dexURL + lastURL
	fmt.Println(loginURL)
	_ = checkErr(client.PostForm(loginURL, url.Values{
		"login":    []string{"admin@epinio.io"},
		"password": []string{"password"},
	}))

	// approve request
	approvalURL := dexURL + lastURL
	fmt.Println(approvalURL)
	res := checkErr(client.PostForm(approvalURL, url.Values{
		"approval": []string{"approve"},
	}))

	// get auth code from HTML
	html := checkErr(io.ReadAll(res.Body))

	for _, line := range strings.Split(string(html), "\n") {
		if strings.Contains(line, "value") {
			return strings.Split(line, `"`)[5]
		}
	}

	return ""
}

func checkErr[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
