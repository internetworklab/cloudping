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
	pkggoogle "github.com/internetworklab/cloudping/pkg/google"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type GoogleOAuthLoginHandler struct {
	SessionLifespan time.Duration

	// Must be specified, get it from Google Cloud Console OAuth 2.0 Client settings
	GoogleOAuthClientId string

	// Must be specified, get it from Google Cloud Console OAuth 2.0 Client settings
	GoogleOAuthClientSecret string

	// Must be specified, one of the authorized redirect URIs for the OAuth 2.0 client
	GoogleOAuthRedirURL string

	// If this is empty, we would use default value (https://accounts.google.com/o/oauth2/v2/auth) for it.
	GoogleOAuthLoginPage string

	// If this is empty, we would use default value ("openid profile email") for it.
	GoogleOAuthScope string

	// If this is empty, we would use default value (https://oauth2.googleapis.com/token) for it.
	GoogleOAuthTokenEndpoint string

	LoginSuccessRedirectURL string

	TokenIssuer pkgauth.JWTIssuer
	NonceIssuer pkgauth.NonceIssuer
}

func (h *GoogleOAuthLoginHandler) getGoogleLoginPage() string {
	if h.GoogleOAuthLoginPage != "" {
		return h.GoogleOAuthLoginPage
	}
	return "https://accounts.google.com/o/oauth2/v2/auth"
}

func (h *GoogleOAuthLoginHandler) getGoogleTokenEndpoint() string {
	if h.GoogleOAuthTokenEndpoint != "" {
		return h.GoogleOAuthTokenEndpoint
	}
	return "https://oauth2.googleapis.com/token"
}

func (h *GoogleOAuthLoginHandler) getGoogleOAuthScope() string {
	if h.GoogleOAuthScope != "" {
		return h.GoogleOAuthScope
	}
	scopes := []string{"openid", "profile", "email"}
	return strings.Join(scopes, " ")
}

func (h *GoogleOAuthLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (h *GoogleOAuthLoginHandler) getGoogleOAuthRedirectURL(nonce string) string {
	urlVals := url.Values{}
	urlVals.Set("client_id", h.GoogleOAuthClientId)
	urlVals.Set("redirect_uri", h.GoogleOAuthRedirURL)
	urlVals.Set("response_type", "code")
	urlVals.Set("scope", h.getGoogleOAuthScope())
	urlVals.Set("state", nonce)
	urlObj, err := url.Parse(h.getGoogleLoginPage())
	if err != nil {
		log.New(os.Stderr, "GoogleLoginHandler", 0).Println("Invalid google login page url:", err)
		return ""
	}
	urlObj.RawQuery = urlVals.Encode()
	return urlObj.String()
}

func (h *GoogleOAuthLoginHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nonce, err := h.NonceIssuer.IssueNonce(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to issue nonce"})
		return
	}

	cookieObj := h.BuildCookieFromToken(DefaultNonceCookieKey, nonce)
	http.SetCookie(w, cookieObj)

	redirURL := h.getGoogleOAuthRedirectURL(nonce)
	if redirURL == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to determine redir url (internal error)"})
		return
	}
	http.Redirect(w, r, redirURL, http.StatusTemporaryRedirect)
}

func (h *GoogleOAuthLoginHandler) handleAuthorizationCode(w http.ResponseWriter, r *http.Request) {
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
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("Nonce is not found from the cookies: %v", err)})
		return
	}
	if nonceFromCookie == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "Nonce is not found from the cookies"})
		return
	}

	if nonceFromCookie.Value != nonce {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "Nonce from the cookie does not match the nonce in the request"})
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
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "No authorization code is found in the request"})
		return
	}

	// Exchange authorization code for tokens
	// See: https://developers.google.com/identity/protocols/oauth2/web-server#httprest_1
	bodyVals := url.Values{}
	bodyVals.Set("client_id", h.GoogleOAuthClientId)
	bodyVals.Set("client_secret", h.GoogleOAuthClientSecret)
	bodyVals.Set("code", authZCode)
	bodyVals.Set("grant_type", "authorization_code")
	bodyVals.Set("redirect_uri", h.GoogleOAuthRedirURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.getGoogleTokenEndpoint(), strings.NewReader(bodyVals.Encode()))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to create request to google token endpoint"})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Failed to get token from google: %+v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tokenErrResp := new(pkggoogle.GoogleTokenErrorResponse)
		if decodeErr := json.NewDecoder(resp.Body).Decode(tokenErrResp); decodeErr != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Token exchange failed with status %d", resp.StatusCode)})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Token exchange failed: %s: %s", tokenErrResp.Error, tokenErrResp.ErrorDescription)})
		return
	}

	tokenResp := new(pkggoogle.GoogleTokenResponse)
	if err := json.NewDecoder(resp.Body).Decode(tokenResp); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to decode google token api response"})
		return
	}

	// Verify that required scopes were granted (Step 6 per Google docs)
	if tokenResp.Scope == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "No scopes were granted by Google"})
		return
	}
	hasEmail := strings.Contains(tokenResp.Scope, "email") || strings.Contains(tokenResp.Scope, "userinfo.email")
	hasProfile := strings.Contains(tokenResp.Scope, "profile") || strings.Contains(tokenResp.Scope, "userinfo.profile")
	if !hasEmail && !hasProfile {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Required scopes (email or profile) were not granted"})
		return
	}

	defer revokeGoogleToken(tokenResp.AccessToken)

	// Fetch user profile using the access token
	profile, err := pkggoogle.GetGoogleProfileByToken(ctx, tokenResp.AccessToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Failed to get Google user profile: %v", err)})
		return
	}

	googleId := profile.Sub
	if googleId == "" {
		googleId = profile.Id
	}
	if googleId == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to get Id of google user"})
		return
	}

	subjectId := fmt.Sprintf("google:%s", googleId)
	username := profile.Email
	if username == "" {
		username = profile.Name
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

	cookieObj := h.BuildCookieFromToken(DefaultJWTCookieKey, token)
	http.SetCookie(w, cookieObj)

	redirUrl := "/"
	if u := h.LoginSuccessRedirectURL; u != "" {
		redirUrl = u
	}

	log.Printf("User %s (id=%s) from google has been successfully logged in, redirecting to %s", username, googleId, redirUrl)
	http.Redirect(w, r, redirUrl, http.StatusTemporaryRedirect)
}

func (h *GoogleOAuthLoginHandler) BuildCookieFromToken(name, value string) *http.Cookie {
	cookieObj := &http.Cookie{}
	cookieObj.HttpOnly = true
	cookieObj.Secure = true
	cookieObj.SameSite = http.SameSiteLaxMode
	cookieObj.Path = "/"
	cookieObj.Name = name
	cookieObj.Value = value
	return cookieObj
}

func (h *GoogleOAuthLoginHandler) GetMapClaims(r *http.Request, subjectId string, username string) (jwt.MapClaims, error) {

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

// revokeGoogleToken revokes the Google OAuth access token.
// See: https://developers.google.com/identity/protocols/oauth2/web-server#tokenrevoke
func revokeGoogleToken(token string) error {
	body := url.Values{}
	body.Set("token", token)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://oauth2.googleapis.com/revoke", strings.NewReader(body.Encode()))
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
		return fmt.Errorf("failed to revoke google token: status code %d", resp.StatusCode)
	}
	return nil
}

func (h *GoogleOAuthLoginHandler) handleNotFoundForThis(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Path %s has no handler attached", r.URL.Path)})
}
