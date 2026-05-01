package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type VisitorLoginHandler struct {
	JWTIssuer       pkgauth.JWTIssuer
	Validity        time.Duration
	TicketGenerator pkgauth.TicketGenerator
}

func (h *VisitorLoginHandler) GetMapClaims(r *http.Request) (jwt.MapClaims, error) {
	visitorId := rand.IntN(65536)
	visitorIdStr := fmt.Sprintf("%05d", visitorId)

	customClaims := &pkgauth.CustomClaimType{}
	customClaims.ID = uuid.NewString()
	customClaims.Subject = uuid.NewString()
	customClaims.Audience = make([]string, 0)
	customClaims.Audience = append(customClaims.Audience, pkgauth.AudSession)
	now := time.Now()
	customClaims.NotBefore = jwt.NewNumericDate(now)
	customClaims.ExpiresAt = jwt.NewNumericDate(now.Add(h.Validity))
	customClaims.Username = "visitor-" + visitorIdStr

	return pkgauth.NewMapClaims(customClaims)
}

func (h *VisitorLoginHandler) BuildCookieFromToken(token string) *http.Cookie {
	cookieObj := &http.Cookie{}
	cookieObj.HttpOnly = true
	cookieObj.Secure = true

	cookieObj.Path = "/"
	cookieObj.Name = DefaultJWTCookieKey
	cookieObj.Value = token
	return cookieObj
}

func (h *VisitorLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, err := h.TicketGenerator.GetTicket(ctx)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("can't wait for visitor ticket to be generated: %w", err).Error()})
		return
	}

	claims, err := h.GetMapClaims(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("failed to generate the token claims for you: %v", err)})
		return
	}

	token, err := h.JWTIssuer.IssueToken(ctx, claims)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("failed to sign token: %v", err)})
		return
	}

	subj := ""
	if s, err := claims.GetSubject(); err == nil {
		subj = s
	}
	log.Printf("issued token for remote %s, subject is %s", pkgutils.GetRemoteAddr(r), subj)

	cookieObj := h.BuildCookieFromToken(token)

	http.SetCookie(w, cookieObj)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
