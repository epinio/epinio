package web

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/internal/domain"
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

type DexController struct {
}

func (c DexController) provider(ctx context.Context) (*oidc.Provider, error) {
	domain, err := domain.MainDomain(ctx)
	if err != nil {
		return nil, err
	}
	providerURL := fmt.Sprintf("https://%s.%s", deployments.DexDeploymentID, domain)

	return oidc.NewProvider(ctx, providerURL)
}

func (c DexController) oauth2Config(ctx context.Context, endpoint oauth2.Endpoint, scopes []string) *oauth2.Config {
	// Configure the OAuth2 config with the client values.
	oauth2Config := oauth2.Config{
		// client_id and client_secret of the client.
		ClientID:     "epinio",
		ClientSecret: "123", // TODO: Put it in a secret. Check also dex.go

		// The redirectURL.
		RedirectURL: "http://127.0.0.1:5555/callback",

		// Discovery returns the OAuth2 endpoints.
		Endpoint: endpoint,

		// "openid" is a required scope for OpenID Connect flows.
		//
		// Other scopes, such as "groups" can be requested.
		Scopes: []string{oidc.ScopeOpenID, "profile", "email", "groups"},
	}

	return &oauth2Config
}

func (c DexController) Login(w http.ResponseWriter, r *http.Request) {
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

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // TODO: Fix this
		},
	}
	ctx := oidc.ClientContext(r.Context(), client)

	provider, err := c.provider(ctx)
	err = errors.Wrap(err, "creating the provider")
	if handleError(w, err, 500) {
		return
	}

	oauth2Config := c.oauth2Config(r.Context(), provider.Endpoint(), scopes)

	var s struct {
		// What scopes does a provider support?
		// See: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
		ScopesSupported []string `json:"scopes_supported"`
	}
	err = errors.Wrap(err, "config")
	if handleError(w, err, 500) {
		return
	}

	err = provider.Claims(&s)
	err = errors.Wrap(err, "claiming")
	if handleError(w, err, 500) {
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

	http.Redirect(w, r, authCodeURL, http.StatusSeeOther)
}

func (c DexController) Callback(w http.ResponseWriter, r *http.Request) {
	var (
		err   error
		token *oauth2.Token
	)

	// TODO: What CA do we need to trust?
	client := &http.Client{
		Transport: debugTransport{http.DefaultTransport},
	}
	ctx := oidc.ClientContext(r.Context(), client)

	provider, err := c.provider(r.Context())
	if handleError(w, err, 500) {
		return
	}
	oauth2Config := c.oauth2Config(r.Context(), provider.Endpoint(), nil)

	switch r.Method {
	case http.MethodGet:
		// Authorization redirect callback from OAuth2 auth flow.
		if errMsg := r.FormValue("error"); errMsg != "" {
			http.Error(w, errMsg+": "+r.FormValue("error_description"), http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		if code == "" {
			http.Error(w, fmt.Sprintf("no code in request: %q", r.Form), http.StatusBadRequest)
			return
		}
		if state := r.FormValue("state"); state != AppState {
			http.Error(w, fmt.Sprintf("expected state %q got %q", AppState, state), http.StatusBadRequest)
			return
		}
		token, err = oauth2Config.Exchange(ctx, code)
	case http.MethodPost:
		// Form request from frontend to refresh a token.
		refresh := r.FormValue("refresh_token")
		if refresh == "" {
			http.Error(w, fmt.Sprintf("no refresh_token in request: %q", r.Form), http.StatusBadRequest)
			return
		}
		t := &oauth2.Token{
			RefreshToken: refresh,
			Expiry:       time.Now().Add(-time.Hour),
		}
		token, err = oauth2Config.TokenSource(ctx, t).Token()
	default:
		http.Error(w, fmt.Sprintf("method not implemented: %s", r.Method), http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get token: %v", err), http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in token response", http.StatusInternalServerError)
		return
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: "epinio"}) // TODO: Don't hardcode
	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to verify ID token: %v", err), http.StatusInternalServerError)
		return
	}

	accessToken, ok := token.Extra("access_token").(string)
	if !ok {
		http.Error(w, "no access_token in token response", http.StatusInternalServerError)
		return
	}

	var claims json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, fmt.Sprintf("error decoding ID token claims: %v", err), http.StatusInternalServerError)
		return
	}

	buff := new(bytes.Buffer)
	if err := json.Indent(buff, []byte(claims), "", "  "); err != nil {
		http.Error(w, fmt.Sprintf("error indenting ID token claims: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, fmt.Sprint(accessToken))
}
