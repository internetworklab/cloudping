package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

const CF_JWT_HEADER = "Cf-Access-Jwt-Assertion"
const CF_AUTH_COOKIE = "CF_AUTHORIZATION"

type WithCloudflareJWTValidate struct {
	CloudflareTeamName string
	CloudflareAUD      string
	Origin             http.Handler
}

func (withCfJWT *WithCloudflareJWTValidate) mustGetTeam() string {
	if team := withCfJWT.CloudflareTeamName; team != "" {
		return team
	}
	log.Panic("Cloudflare team name not specified")
	return ""
}

func (withCfJWT *WithCloudflareJWTValidate) mustGetPubkeysURL() string {
	team := withCfJWT.mustGetTeam()
	urlStr := fmt.Sprintf("https://%s.cloudflareaccess.com/cdn-cgi/access/certs", team)
	return urlStr
}

func (withCftJWT *WithCloudflareJWTValidate) mustGetAUD() string {
	if aud := withCftJWT.CloudflareAUD; aud != "" {
		return aud
	}
	log.Panic("Cloudflare AUD not specified")
	return ""
}

func (withCfgJWT *WithCloudflareJWTValidate) mustGetVerifier(ctx context.Context) *oidc.IDTokenVerifier {

	config := &oidc.Config{
		ClientID: withCfgJWT.mustGetAUD(),
	}
	keySet := oidc.NewRemoteKeySet(ctx, withCfgJWT.mustGetPubkeysURL())
	teamDomain := fmt.Sprintf("https://%s.cloudflareaccess.com", withCfgJWT.mustGetTeam())
	return oidc.NewVerifier(teamDomain, keySet, config)
}

func (handler *WithCloudflareJWTValidate) getCFJWT(r *http.Request) string {
	if accessJWT := r.Header.Get(CF_JWT_HEADER); accessJWT != "" {
		return accessJWT
	}

	if cookieObj, err := r.Cookie(CF_AUTH_COOKIE); err == nil && cookieObj != nil {
		if accessJWT := cookieObj.Value; accessJWT != "" {
			return accessJWT
		}
	}
	return ""
}

func (handler *WithCloudflareJWTValidate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	accessJWT := handler.getCFJWT(r)
	if accessJWT == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "No token on the request"})
		return
	}

	verifier := handler.mustGetVerifier(ctx)
	idToken, err := verifier.Verify(ctx, accessJWT)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("Invalid token: %s", err.Error())})
		return
	}

	if idToken == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: "IdToken is nil"})
		return
	}

	// mapClaims := jwt.MapClaims{}
	// if err := idToken.Claims(&mapClaims); err != nil {
	// 	log.Panic("Can not unmarshal id token claims")
	// }

	// for k, v := range mapClaims {
	// 	log.Printf("Found claim %s: %v", k, v)
	// }

	handler.Origin.ServeHTTP(w, r)
}
