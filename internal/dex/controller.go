package dex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/epinio/epinio/internal/domain"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// TODO: https://dexidp.io/docs/using-dex/#state-tokens
const AppState = "TODO: generate this"

type debugTransport struct {
	t http.RoundTripper
}

func (d debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, err
	}
	log.Printf("%s", reqDump)

	resp, err := d.t.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	log.Printf("%s", respDump)
	return resp, nil
}

type Controller struct {
}

func (c Controller) oauth2Config(ctx context.Context, endpoint oauth2.Endpoint, scopes []string) (*oauth2.Config, error) {
	domain, err := domain.MainDomain(ctx)
	if err != nil {
		return nil, err
	}

	// Configure the OAuth2 config with the client values.
	oauth2Config := oauth2.Config{
		// client_id and client_secret of the client.
		ClientID:     "epinio-ui",                    // TODO: When should the cli be used? Maybe use a request param?
		ClientSecret: os.Getenv("DEX_CLIENT_SECRET"), // TODO: Create an `epinio server` command line argument for it?

		// The redirectURL.
		RedirectURL: fmt.Sprintf("https://epinio.%s/dex/callback", domain),

		// Discovery returns the OAuth2 endpoints.
		Endpoint: endpoint,

		// "openid" is a required scope for OpenID Connect flows.
		//
		// Other scopes, such as "groups" can be requested.
		Scopes: []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}

	return &oauth2Config, nil
}

func (c Controller) Login(ctx *gin.Context) {
	r := ctx.Request
	var scopes []string
	if extraScopes := r.FormValue("extra_scopes"); extraScopes != "" {
		scopes = strings.Split(extraScopes, " ")
	}
	var clients []string
	if crossClients := r.FormValue("cross_client"); crossClients != "" {
		clients = strings.Split(crossClients, " ")
	}
	for _, client := range clients {
		scopes = append(scopes, "audience:server:client_id:"+client)
	}
	connectorID := ""
	if id := r.FormValue("connector_id"); id != "" {
		connectorID = id
	}

	client, err := httpClientForRootCAs("/dex-tls/ca.crt")
	if handleErr(err, http.StatusInternalServerError, "creating a client that trusts dex certificate", ctx) {
		return
	}

	provider, err := provider(oidc.ClientContext(ctx, client))
	if handleErr(err, http.StatusInternalServerError, "creating the provider", ctx) {
		return
	}

	oauth2Config, err := c.oauth2Config(ctx, provider.Endpoint(), scopes)
	if handleErr(err, http.StatusInternalServerError, "creating the oauth2Config", ctx) {
		return
	}

	var s struct {
		// What scopes does a provider support?
		// See: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
		ScopesSupported []string `json:"scopes_supported"`
	}

	err = provider.Claims(&s)
	if handleErr(err, http.StatusInternalServerError, "finding claims", ctx) {
		return
	}

	var offlineAsScope bool
	if len(s.ScopesSupported) == 0 {
		// scopes_supported is a "RECOMMENDED" discovery claim, not a required
		// one. If missing, assume that the provider follows the spec and has
		// an "offline_access" scope.
		offlineAsScope = true
	} else {
		// See if scopes_supported has the "offline_access" scope.
		offlineAsScope = func() bool {
			for _, scope := range s.ScopesSupported {
				if scope == oidc.ScopeOfflineAccess {
					return true
				}
			}
			return false
		}()
	}

	authCodeURL := ""
	scopes = append(scopes, "openid", "profile", "email")
	if r.FormValue("offline_access") != "yes" {
		authCodeURL = oauth2Config.AuthCodeURL(AppState)
	} else if offlineAsScope {
		scopes = append(scopes, "offline_access")
		authCodeURL = oauth2Config.AuthCodeURL(AppState)
	} else {
		authCodeURL = oauth2Config.AuthCodeURL(AppState, oauth2.AccessTypeOffline)
	}
	if connectorID != "" {
		authCodeURL = authCodeURL + "&connector_id=" + connectorID
	}

	http.Redirect(ctx.Writer, r, authCodeURL, http.StatusSeeOther)
}

func (c Controller) Callback(ctx *gin.Context) {
	var (
		err   error
		token *oauth2.Token
	)

	r := ctx.Request

	client, err := httpClientForRootCAs("/dex-tls/ca.crt")
	if handleErr(err, http.StatusInternalServerError, "creating a client that trusts dex certificate", ctx) {
		return
	}

	provider, err := provider(oidc.ClientContext(r.Context(), client))
	if handleErr(err, http.StatusInternalServerError, "setting up the provider", ctx) {
		return
	}

	oauth2Config, err := c.oauth2Config(ctx, provider.Endpoint(), nil)
	if handleErr(err, http.StatusInternalServerError, "creating the oauth2Config", ctx) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Authorization redirect callback from OAuth2 auth flow.
		if errMsg := r.FormValue("error"); errMsg != "" {
			handleErr(
				errors.New(errMsg+": "+r.FormValue("error_description")),
				http.StatusBadRequest, "", ctx)
			return
		}
		code := r.FormValue("code")
		if code == "" {
			handleErr(
				errors.Errorf("no code in request: %q", r.Form),
				http.StatusBadRequest, "", ctx)
			return
		}
		if state := r.FormValue("state"); state != AppState {
			handleErr(
				errors.Errorf("expected state %q got %q", AppState, state),
				http.StatusBadRequest, "", ctx)
			return
		}

		token, err = oauth2Config.Exchange(oidc.ClientContext(ctx, client), code)
		if handleErr(err, http.StatusBadRequest, "failed to get token", ctx) {
			return
		}
	case http.MethodPost:
		// Form request from frontend to refresh a token.
		refresh := r.FormValue("refresh_token")
		if refresh == "" {
			handleErr(
				errors.Errorf("no refresh_token in request: %q", r.Form),
				http.StatusBadRequest, "", ctx)
			return
		}
		t := &oauth2.Token{
			RefreshToken: refresh,
			Expiry:       time.Now().Add(-time.Hour),
		}
		token, err = oauth2Config.TokenSource(oidc.ClientContext(ctx, client), t).Token()
		if handleErr(err, http.StatusBadRequest, "failed to get token", ctx) {
			return
		}
	default:
		handleErr(errors.Errorf("method not implemented: %s", r.Method),
			http.StatusBadRequest, "", ctx)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		handleErr(
			errors.New("no id_token in token response"),
			http.StatusInternalServerError, "", ctx)
		return
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: "epinio-ui"})
	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if handleErr(err,
		http.StatusInternalServerError, "failed to verify ID token", ctx) {
		return
	}

	accessToken, ok := token.Extra("access_token").(string)
	if !ok {
		handleErr(
			errors.New("no access_token in token response"),
			http.StatusInternalServerError, "", ctx)
		return
	}

	var claims json.RawMessage
	err = idToken.Claims(&claims)
	if handleErr(err,
		http.StatusInternalServerError, "error decoding ID token claims", ctx) {
		return
	}

	buff := new(bytes.Buffer)
	err = json.Indent(buff, []byte(claims), "", "  ")
	if handleErr(err,
		http.StatusInternalServerError, "error indenting ID token claims", ctx) {
		return
	}

	session := sessions.Default(ctx)
	session.Set("dex-token", accessToken)
	ctx.Redirect(http.StatusSeeOther, "/")
}

// If err is not nil, it prepares the response and returns true.
// It returns false if there was no error.
func handleErr(err error, statusCode int, message string, ctx *gin.Context) bool {
	if err == nil {
		return false
	}

	ctx.String(statusCode, "%s", errors.Wrap(err, message).Error())

	return true
}
