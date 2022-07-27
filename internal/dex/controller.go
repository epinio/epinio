package dex

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

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/epinio/epinio/internal/domain"
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

func (c Controller) provider(ctx context.Context) (*oidc.Provider, error) {
	domain, err := domain.MainDomain(ctx)
	if err != nil {
		return nil, err
	}
	providerURL := fmt.Sprintf("https://dex.%s", domain)

	return oidc.NewProvider(ctx, providerURL)
}

func (c Controller) oauth2Config(ctx context.Context, endpoint oauth2.Endpoint, scopes []string) (*oauth2.Config, error) {
	domain, err := domain.MainDomain(ctx)
	if err != nil {
		return nil, err
	}

	// Configure the OAuth2 config with the client values.
	oauth2Config := oauth2.Config{
		// client_id and client_secret of the client.
		ClientID:     "epinio-ui", // TODO: When should the cli be used? Maybe use a request param?
		ClientSecret: "123",       // TODO: Read it from the secret

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

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // TODO: Fix this
		},
	}

	provider, err := c.provider(oidc.ClientContext(ctx, client))
	if err != nil {
		ctx.Error(errors.Wrap(err, "creating the provider"))
	}

	oauth2Config, err := c.oauth2Config(ctx, provider.Endpoint(), scopes)
	if err != nil {
		ctx.Error(errors.Wrap(err, "creating the oauth2Config"))
	}

	var s struct {
		// What scopes does a provider support?
		// See: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
		ScopesSupported []string `json:"scopes_supported"`
	}
	err = errors.Wrap(err, "config")
	if err != nil {
		ctx.Error(err)
	}

	err = provider.Claims(&s)
	if err != nil {
		ctx.Error(err)
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

func (c Controller) Callback(ctx *gin.Context) apierror.APIErrors {
	var (
		err   error
		token *oauth2.Token
	)

	r := ctx.Request
	w := ctx.Writer

	// TODO: What CA do we need to trust?
	client := &http.Client{
		Transport: debugTransport{http.DefaultTransport},
	}

	provider, err := c.provider(oidc.ClientContext(r.Context(), client))
	if err != nil {
		return apierror.InternalError(err, "claiming")
	}

	oauth2Config, err := c.oauth2Config(ctx, provider.Endpoint(), nil)
	if err != nil {
		return apierror.InternalError(err, "creating the oauth2Config")
	}

	switch r.Method {
	case http.MethodGet:
		// Authorization redirect callback from OAuth2 auth flow.
		if errMsg := r.FormValue("error"); errMsg != "" {
			return apierror.NewBadRequestError(errMsg + ": " + r.FormValue("error_description"))
		}
		code := r.FormValue("code")
		if code == "" {
			return apierror.NewBadRequestError(fmt.Sprintf("no code in request: %q", r.Form))
		}
		if state := r.FormValue("state"); state != AppState {
			return apierror.NewBadRequestError(fmt.Sprintf("expected state %q got %q", AppState, state))
		}
		token, err = oauth2Config.Exchange(ctx, code)
	case http.MethodPost:
		// Form request from frontend to refresh a token.
		refresh := r.FormValue("refresh_token")
		if refresh == "" {
			return apierror.NewBadRequestError(fmt.Sprintf("no refresh_token in request: %q", r.Form))
		}
		t := &oauth2.Token{
			RefreshToken: refresh,
			Expiry:       time.Now().Add(-time.Hour),
		}
		token, err = oauth2Config.TokenSource(ctx, t).Token()
	default:
		return apierror.NewBadRequestError(fmt.Sprintf("method not implemented: %s", r.Method))
	}

	if err != nil {
		return apierror.NewInternalError(fmt.Sprintf("failed to get token: %v", err))
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return apierror.NewInternalError("no id_token in token response")
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: "epinio"}) // TODO: Don't hardcode
	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		return apierror.NewInternalError(fmt.Sprintf("failed to verify ID token: %v", err))
	}

	accessToken, ok := token.Extra("access_token").(string)
	if !ok {
		return apierror.NewInternalError("no access_token in token response")
	}

	var claims json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		return apierror.NewInternalError(fmt.Sprintf("error decoding ID token claims: %v", err))
	}

	buff := new(bytes.Buffer)
	if err := json.Indent(buff, []byte(claims), "", "  "); err != nil {
		return apierror.NewInternalError(fmt.Sprintf("error indenting ID token claims: %v", err))
	}

	fmt.Fprint(w, fmt.Sprint(accessToken))

	return nil
}
