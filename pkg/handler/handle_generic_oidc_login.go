package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkgoidc "github.com/internetworklab/cloudping/pkg/oidc"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

// GenericOIDCLoginHandler implements the OAuth 2.0 Authorization Code flow
// with OpenID Connect extensions against any OIDC-compliant provider.
// Endpoints are discovered automatically from the provider's
// {.well-known/openid-configuration} document.
//
// Register this handler on a path ending with "/", e.g. "/login/as/oidc/".
// Sub-paths "/start" and "/auth" are handled automatically.
type GenericOIDCLoginHandler struct {
	SessionLifespan time.Duration

	// A short name identifying the OIDC provider (e.g. "keycloak", "auth0").
	// Used as a prefix in the subject claim: "oidc-{ProviderName}:{sub}".
	// Defaults to "oidc" if empty.
	ProviderName string

	// The issuer URL of the OIDC provider (e.g. "https://auth.example.com/realms/myrealm").
	// The discovery document is fetched from {IssuerURL}/.well-known/openid-configuration.
	IssuerURL string

	// The OAuth 2.0 client ID registered with the OIDC provider.
	ClientId string

	// The OAuth 2.0 client secret registered with the OIDC provider.
	ClientSecret string

	// One of the authorized redirect URIs for the OAuth 2.0 client.
	RedirectURL string

	// Space-delimited scopes. Defaults to "openid profile email" if empty.
	Scope string

	LoginSuccessRedirectURL string

	TokenIssuer pkgauth.JWTIssuer
	NonceIssuer pkgauth.NonceIssuer

	discoveryCache *pkgoidc.DiscoveryCache
	providerCache  *pkgoidc.ProviderCache
}

const defaultScope = "openid profile email"

func (h *GenericOIDCLoginHandler) getScope() string {
	if h.Scope != "" {
		return h.Scope
	}
	return defaultScope
}

func (h *GenericOIDCLoginHandler) getProviderName() string {
	if h.ProviderName != "" {
		return h.ProviderName
	}
	return "oidc"
}

func (h *GenericOIDCLoginHandler) getDiscovery(ctx context.Context) (*pkgoidc.DiscoveryDocument, error) {
	if h.discoveryCache == nil {
		h.discoveryCache = pkgoidc.NewDiscoveryCache(h.IssuerURL, 1*time.Hour)
	}
	return h.discoveryCache.Get(ctx)
}

func (h *GenericOIDCLoginHandler) getProviderCache() *pkgoidc.ProviderCache {
	if h.providerCache == nil {
		h.providerCache = pkgoidc.NewProviderCache(h.IssuerURL)
	}
	return h.providerCache
}

func (h *GenericOIDCLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if reqUrl := r.URL; reqUrl != nil {
		if strings.HasSuffix(reqUrl.Path, "/start") {
			h.handleStart(w, r)
			return
		} else if strings.HasSuffix(reqUrl.Path, "/auth") {
			h.handleAuthorizationCode(w, r)
			return
		}
	}
	h.handleNotFoundForThis(w, r)
}

func (h *GenericOIDCLoginHandler) buildAuthorizeURL(nonce, authzEndpoint string) string {
	urlVals := url.Values{}
	urlVals.Set("client_id", h.ClientId)
	urlVals.Set("redirect_uri", h.RedirectURL)
	urlVals.Set("response_type", "code")
	urlVals.Set("scope", h.getScope())
	urlVals.Set("state", nonce)
	urlVals.Set("nonce", nonce)
	urlObj, err := url.Parse(authzEndpoint)
	if err != nil {
		log.New(os.Stderr, "GenericOIDCLoginHandler", 0).Printf("Invalid authorization endpoint URL: %v", err)
		return ""
	}
	urlObj.RawQuery = urlVals.Encode()
	return urlObj.String()
}

func (h *GenericOIDCLoginHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	disc, err := h.getDiscovery(ctx)
	if err != nil {
		log.Printf("Failed to fetch OIDC discovery document for %q: %v", h.IssuerURL, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to fetch OIDC provider configuration"})
		return
	}

	nonce, err := h.NonceIssuer.IssueNonce(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to issue nonce"})
		return
	}

	http.SetCookie(w, h.BuildCookieFromToken(DefaultNonceCookieKey, nonce))

	redirURL := h.buildAuthorizeURL(nonce, disc.AuthorizationEndpoint)
	if redirURL == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to build authorization URL (internal error)"})
		return
	}
	http.Redirect(w, r, redirURL, http.StatusTemporaryRedirect)
}

func (h *GenericOIDCLoginHandler) handleAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	// See rfc6749 section-4.1.2 and section-4.1.2.1
	// https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("%s: %s", errParam, errDesc)})
		return
	}

	ctx := r.Context()
	nonce := r.URL.Query().Get("state")
	nonceFromCookie, err := r.Cookie(DefaultNonceCookieKey)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("Nonce not found in cookies: %v", err)})
		return
	}
	if nonceFromCookie == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "Nonce not found in cookies"})
		return
	}

	if nonceFromCookie.Value != nonce {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "Nonce from cookie does not match nonce in request"})
		return
	}

	valid, err := h.NonceIssuer.ValidateNonce(ctx, nonce)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Invalid nonce: %v", err)})
		return
	}
	if !valid {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Invalid nonce"})
		return
	}

	// Clear the nonce cookie after successful validation
	http.SetCookie(w, &http.Cookie{
		Name:     DefaultNonceCookieKey,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	authZCode := r.URL.Query().Get("code")
	if authZCode == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "No authorization code found in request"})
		return
	}

	// Fetch discovery document to get the token endpoint
	disc, err := h.getDiscovery(ctx)
	if err != nil {
		log.Printf("Failed to fetch OIDC discovery document for %q: %v", h.IssuerURL, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to fetch OIDC provider configuration"})
		return
	}

	// Exchange authorization code for tokens
	bodyVals := url.Values{}
	bodyVals.Set("client_id", h.ClientId)
	bodyVals.Set("client_secret", h.ClientSecret)
	bodyVals.Set("code", authZCode)
	bodyVals.Set("grant_type", "authorization_code")
	bodyVals.Set("redirect_uri", h.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, disc.TokenEndpoint, strings.NewReader(bodyVals.Encode()))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to create token exchange request"})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Failed to exchange token: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tokenErrResp := new(pkgoidc.TokenErrorResponse)
		if decodeErr := json.NewDecoder(resp.Body).Decode(tokenErrResp); decodeErr != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Token exchange failed with status %d", resp.StatusCode)})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Token exchange failed: %s: %s", tokenErrResp.Error, tokenErrResp.ErrorDescription)})
		return
	}

	tokenResp := new(pkgoidc.TokenResponse)
	if err := json.NewDecoder(resp.Body).Decode(tokenResp); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to decode token response"})
		return
	}

	// Verify the ID token (signature, issuer, audience, expiry, nonce).
	// The ID token is the primary identity artifact in OIDC.
	if tokenResp.IdToken == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "No ID token in token response from OIDC provider"})
		return
	}

	idToken, err := pkgoidc.VerifyIDToken(ctx, h.getProviderCache(), h.ClientId, tokenResp.IdToken, nonce)
	if err != nil {
		log.Printf("ID token verification failed for OIDC provider %q: %v", h.getProviderName(), err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("ID token verification failed: %v", err)})
		return
	}

	// Revoke the access token if the provider exposes a revocation endpoint
	if disc.RevocationEndpoint != "" {
		defer revokeOIDCToken(disc.RevocationEndpoint, tokenResp.AccessToken)
	}

	// Extract user identity from the verified ID token claims first.
	idTokenClaims := new(pkgoidc.UserInfoResponse)
	if err := idToken.Claims(idTokenClaims); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Failed to extract ID token claims: %v", err)})
		return
	}

	userId := idTokenClaims.Sub
	if userId == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to get user ID (sub claim) from ID token"})
		return
	}

	subjectId := fmt.Sprintf("oidc-%s:%s", h.getProviderName(), userId)

	username := idTokenClaims.PreferredUsername
	if username == "" {
		username = idTokenClaims.Email
	}
	if username == "" {
		username = idTokenClaims.Name
	}

	// Enrich profile from the userinfo endpoint if the provider exposes one.
	if disc.UserInfoEndpoint != "" {
		userinfo, err := pkgoidc.FetchUserInfo(ctx, disc.UserInfoEndpoint, tokenResp.AccessToken)
		if err != nil {
			log.Printf("Failed to fetch userinfo from %q (non-fatal): %v", disc.UserInfoEndpoint, err)
		} else {
			if userinfo.Sub != "" && userinfo.Sub != userId {
				log.Printf("userinfo sub %q differs from ID token sub %q", userinfo.Sub, userId)
			}
			if username == "" {
				username = userinfo.PreferredUsername
			}
			if username == "" {
				username = userinfo.Email
			}
			if username == "" {
				username = userinfo.Name
			}
		}
	}

	claims, err := h.GetMapClaims(r, subjectId, username)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Failed to get claims: %v", err)})
		return
	}

	token, err := h.TokenIssuer.IssueToken(ctx, claims)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Failed to issue token: %v", err)})
		return
	}

	http.SetCookie(w, h.BuildCookieFromToken(DefaultJWTCookieKey, token))

	redirUrl := "/"
	if u := h.LoginSuccessRedirectURL; u != "" {
		redirUrl = u
	}

	log.Printf("User %s (id=%s) from OIDC provider %q has been successfully logged in, redirecting to %s", username, userId, h.getProviderName(), redirUrl)
	http.Redirect(w, r, redirUrl, http.StatusTemporaryRedirect)
}

func (h *GenericOIDCLoginHandler) BuildCookieFromToken(name, value string) *http.Cookie {
	cookieObj := &http.Cookie{}
	cookieObj.HttpOnly = true
	cookieObj.Secure = true
	cookieObj.SameSite = http.SameSiteLaxMode
	cookieObj.Path = "/"
	cookieObj.Name = name
	cookieObj.Value = value
	return cookieObj
}

func (h *GenericOIDCLoginHandler) GetMapClaims(r *http.Request, subjectId string, username string) (jwt.MapClaims, error) {

	customClaims := &pkgauth.CustomClaimType{}
	customClaims.ID = uuid.NewString()
	customClaims.Subject = subjectId
	customClaims.Audience = make([]string, 0)
	customClaims.Audience = append(customClaims.Audience, pkgauth.AudSession)
	now := time.Now()
	customClaims.NotBefore = jwt.NewNumericDate(now)
	customClaims.ExpiresAt = jwt.NewNumericDate(now.Add(h.SessionLifespan))
	customClaims.Username = username

	return pkgauth.NewMapClaims(customClaims)
}

// revokeOIDCToken revokes the OAuth access token at the provider's revocation endpoint.
// Uses context.Background() because it is typically called via defer after the response
// has already been written.
func revokeOIDCToken(revocationEndpoint, token string) error {
	body := url.Values{}
	body.Set("token", token)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, revocationEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to revoke OIDC token: status code %d", resp.StatusCode)
	}
	return nil
}

func (h *GenericOIDCLoginHandler) handleNotFoundForThis(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Path %s has no handler attached", r.URL.Path)})
}
