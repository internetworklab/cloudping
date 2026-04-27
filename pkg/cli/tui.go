package cli

import (
	"fmt"
	"net/http"
	"os"

	pkghandler "github.com/internetworklab/cloudping/pkg/handler"
	pkgtuidatasource "github.com/internetworklab/cloudping/pkg/tui/datasource"
	pkgtuihandler "github.com/internetworklab/cloudping/pkg/tui/handler"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type TUICmd struct {
	ListenAddress            string               `name:"listen-address" help:"Address to listen on." type:"string" default:":8084"`
	Authentication           AuthenticationMethod `name:"authentication" help:"Specify the authentication method to use, currently supported: none, cloudflare" default:"none"`
	PingResolver             string               `name:"ping-resolver" help:"Resolver being used to resolver hostname to IP address during an ICMP ping or traceroute task" default:"172.20.0.53:53"`
	UpstreamJWTSecretFromEnv string               `name:"upstream-jwt-sec-env" help:"Name of the enviornment variable that stores the JWT token use to authenticate with the upstream ping events provider" default:"UPSTREAM_JWT_TOKEN"`
	UpstreamAPIPrefix        string               `name:"upstream-api-prefix" help:"The API prefix of the upstream server where to get ping events data" default:"https://ping2.sh/api"`
	CloudflareTeamName       string               `name:"cloudflare-team-name" help:"The name of the Cloudflare team to use for authentication, it must be specified when using --authentication=cloudflare" default:""`
	CloudflareAUDEnv         string               `name:"cloudflare-aud-env" help:"The name of the environment variable of the Application Audience (AUD) tag for your application, it must be specified when using --authentication=cloudflare, see https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/authorization-cookie/validating-json/#get-your-aud-tag"`
}

func (tuiCmd *TUICmd) Run(sharedCtx *pkgutils.GlobalSharedContext) error {
	pingEVProvider := &pkgtuidatasource.CloudPingEventsProvider{
		APIPrefix: tuiCmd.UpstreamAPIPrefix,
		JWTToken:  os.Getenv(tuiCmd.UpstreamJWTSecretFromEnv),
		Resolver:  tuiCmd.PingResolver,
	}

	mux := http.NewServeMux()
	cliHTTPHandler := &pkgtuihandler.CLIHandler{
		LocationsProvider:   pingEVProvider,
		PingEventsProvider:  pingEVProvider,
		GlobalSharedContext: sharedCtx,
	}
	emailCLIHTTPHandler := &pkgtuihandler.WithEmailHandler{
		Next: cliHTTPHandler,
	}

	mux.Handle("/cli", cliHTTPHandler)
	mux.Handle("/mail", emailCLIHTTPHandler)

	var handler http.Handler = mux

	if tuiCmd.Authentication == AuthenticationMethodCloudflare {
		cfValidateMiddleware := &pkghandler.WithCloudflareJWTValidate{
			CloudflareTeamName: tuiCmd.CloudflareTeamName,
			CloudflareAUD:      os.Getenv(tuiCmd.CloudflareAUDEnv),
			Origin:             handler,
		}
		handler = cfValidateMiddleware
	}

	fmt.Printf("Starting HTTP server on %s\n", tuiCmd.ListenAddress)
	return http.ListenAndServe(tuiCmd.ListenAddress, handler)
}
