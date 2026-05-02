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

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type EntraIDLoginHandler struct {
	SessionLifespan time.Duration

	// Must be specified. Use "common" for multi-tenant, or your specific tenant ID.
	EntraIDTenantID     string
	EntraIDClientID     string
	EntraIDClientSecret string
	EntraIDRedirURL     string

	// If empty, defaults to "openid profile email"
	EntraIDScope string

	LoginSuccessRedirectURL string
	TokenIssuer             pkgauth.JWTIssuer
	NonceIssuer             pkgauth.NonceIssuer

	// MSAL confidential client. Prefer injecting this from outside (e.g., main.go)
	// so construction errors are handled at startup, not inside HTTP handlers.
	MSALClient confidential.Client

	// A short name identifying the OIDC provider (e.g. "keycloak", "auth0", "entra").
	// Used as a prefix in the subject claim: "oidc-{ProviderName}:{sub}".
	// Defaults to "entra" if empty.
	// An example usage is that, we could have multiple Entra ID handler endpoint, each with different Entra ID parameters.
	ProviderName string
}

func (h *EntraIDLoginHandler) getProviderName() string {
	if h.ProviderName != "" {
		return h.ProviderName
	}
	return "entra"
}

func (h *EntraIDLoginHandler) getScopes() []string {
	if h.EntraIDScope != "" {
		return strings.Split(h.EntraIDScope, " ")
	}
	return []string{"openid", "profile", "email"}
}

func (h *EntraIDLoginHandler) getEntraIDAuthURL(ctx context.Context, nonce string) (string, error) {
	// 1. 让 MSAL 生成基础授权 URL（不含 state，confidential client 也不含 PKCE）
	baseURL, err := h.MSALClient.AuthCodeURL(ctx, h.EntraIDClientID, h.EntraIDRedirURL, h.getScopes())
	if err != nil {
		return "", fmt.Errorf("msal AuthCodeURL: %w", err)
	}

	// 2. 解析 URL，提取结构化的 query values
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse msal auth url: %w", err)
	}

	// 3. 在结构化的 values 上注入我们自己的参数
	q := u.Query()
	q.Set("state", nonce)
	// Entra ID 默认可能走 form_post，但我们从 r.URL.Query() 读 code，必须强制 query
	q.Set("response_mode", "query")

	// 4. 重新 encode 回 URL
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (h *EntraIDLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (h *EntraIDLoginHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nonce, err := h.NonceIssuer.IssueNonce(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to issue nonce"})
		return
	}

	cookieObj := h.BuildCookieFromToken(DefaultNonceCookieKey, nonce)
	http.SetCookie(w, cookieObj)

	redirURL, err := h.getEntraIDAuthURL(ctx, nonce)
	if err != nil {
		log.New(os.Stderr, "EntraIDLoginHandler", 0).Println("Failed to build auth URL:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to determine redir url (internal error)"})
		return
	}

	http.Redirect(w, r, redirURL, http.StatusTemporaryRedirect)
}

func (h *EntraIDLoginHandler) handleAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	// Entra ID error handling per OAuth2 spec
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

	// Use MSAL to exchange code for tokens. This replaces the manual http.PostForm
	// you do for Google, and gives you caching + refresh for free.
	scopes := h.getScopes()
	result, err := h.MSALClient.AcquireTokenByAuthCode(ctx, authZCode, h.EntraIDRedirURL, scopes)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Failed to exchange code for token: %v", err)})
		return
	}

	// Entra ID uses "oid" (Object ID) as the stable user identifier; fallback to "sub"
	entraUserID := result.IDToken.Oid
	if entraUserID == "" {
		entraUserID = result.IDToken.Subject
	}

	// DONT do the following, becase nor preferred_name or email is considered a stable user identifier,
	// so, if we can't get something that looks like a stable user identifer, we reject the login.
	// if entraUserID == "" {
	// 	entraUserID = result.IDToken.PreferredUsername
	// }
	// if entraUserID == "" {
	// 	entraUserID = result.IDToken.Email
	// }

	if entraUserID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: "Failed to get stable user ID (e.g. oid/sub) from Entra ID token"})
		return
	}

	email := result.IDToken.Email
	name := result.IDToken.PreferredUsername

	subjectId := fmt.Sprintf("%s:%s", h.getProviderName(), entraUserID)
	username := email
	if username == "" {
		username = name
	}

	jwtClaims, err := h.GetMapClaims(r, subjectId, username)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Failed to get claims: %v", err)})
		return
	}

	token, err := h.TokenIssuer.IssueToken(ctx, jwtClaims)
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

	log.Printf("User %s (id=%s) from entra id has been successfully logged in, redirecting to %s", username, entraUserID, redirUrl)
	http.Redirect(w, r, redirUrl, http.StatusTemporaryRedirect)
}

func (h *EntraIDLoginHandler) BuildCookieFromToken(name, value string) *http.Cookie {
	cookieObj := &http.Cookie{}
	cookieObj.HttpOnly = true
	cookieObj.Secure = true
	cookieObj.SameSite = http.SameSiteLaxMode
	cookieObj.Path = "/"
	cookieObj.Name = name
	cookieObj.Value = value
	return cookieObj
}

func (h *EntraIDLoginHandler) GetMapClaims(r *http.Request, subjectId string, username string) (jwt.MapClaims, error) {
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

func (h *EntraIDLoginHandler) handleNotFoundForThis(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(&pkgutils.ErrorResponse{Error: fmt.Sprintf("Path %s has no handler attached", r.URL.Path)})
}
