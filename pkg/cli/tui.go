package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type AuthenticationMethod string

const AuthenticationMethodCloudflare AuthenticationMethod = "cloudflare"
const AuthenticationMethodNone AuthenticationMethod = "none"

type TUICmd struct {
	ListenAddress            string               `name:"listen-address" help:"Address to listen on." type:"string" default:":8084"`
	Authentication           AuthenticationMethod `name:"authentication" help:"Specify the authentication method to use, currently supported: none, cloudflare" default:"none"`
	PingResolver             string               `name:"ping-resolver" help:"Resolver being used to resolver hostname to IP address during an ICMP ping or traceroute task" default:"172.20.0.53:53"`
	UpstreamJWTSecretFromEnv string               `name:"upstream-jwt-sec-env" help:"Name of the enviornment variable that stores the JWT token use to authenticate with the upstream ping events provider" default:"UPSTREAM_JWT_TOKEN"`
	UpstreamAPIPrefix        string               `name:"upstream-api-prefix" help:"The API prefix of the upstream server where to get ping events data" default:"https://ping2.sh/api"`
	CloudflareTeamName       string               `name:"cloudflare-team-name" help:"The name of the Cloudflare team to use for authentication, it must be specified when using --authentication=cloudflare" default:""`
	CloudflareAUDEnv         string               `name:"cloudflare-aud-env" help:"The name of the environment variable of the Application Audience (AUD) tag for your application, it must be specified when using --authentication=cloudflare, see https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/validating-json/#get-your-aud-tag"`
}

const CF_JWT_HEADER = "Cf-Access-Jwt-Assertion"

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
	if accessJWT := r.Header.Get("Cf-Access-Jwt-Assertion"); accessJWT != "" {
		return accessJWT
	}

	if cookieObj, err := r.Cookie("CF_AUTHORIZATION"); err == nil && cookieObj != nil {
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

func (tuiCmd *TUICmd) Run() error {
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("--- Request: %s %s ---\n", r.Method, r.URL.Path)
		for key, vals := range r.Header {
			fmt.Printf("%s: %s\n", key, strings.Join(vals, ", "))
		}
		fmt.Println()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK\n"))
	})

	if tuiCmd.Authentication == AuthenticationMethodCloudflare {
		cfValidateMiddleware := &WithCloudflareJWTValidate{
			CloudflareTeamName: tuiCmd.CloudflareTeamName,
			CloudflareAUD:      os.Getenv(tuiCmd.CloudflareAUDEnv),
			Origin:             handler,
		}
		handler = cfValidateMiddleware
	}

	fmt.Printf("Starting HTTP server on %s\n", tuiCmd.ListenAddress)
	return http.ListenAndServe(tuiCmd.ListenAddress, handler)
}
