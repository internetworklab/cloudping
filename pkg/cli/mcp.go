package cli

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	pkgaihandlers "github.com/internetworklab/cloudping/pkg/ai/handlers"
	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkghandler "github.com/internetworklab/cloudping/pkg/handler"
	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	pkgbotdata "github.com/internetworklab/cloudping/pkg/tui/datasource"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServerCmd struct {
	MCPServerName       string `name:"mcp-server-name" help:"Name for introducting the server itself" default:"cloudping-mcp"`
	MCPServerTitle      string `name:"mcp-server-title" help:"Title for introducing itself to the client as a mcp server" default:"CloudPing"`
	MCPServerWebsiteURL string `name:"mcp-server-website-url" help:"WebsiteURL for introducing itself to the client as a mcp server" default:"https://github.com/internetworklab/cloudping"`

	ListenAddress            string               `name:"listen-address" help:"Address to listen on." type:"string" default:":8090"`
	Authentication           AuthenticationMethod `name:"authentication" help:"Specify the authentication method to use, currently supported: none, jwt. For 'jwt' authentication, attach the jwt token in the Authorization header as 'Authorization: bearer <jwt>'" default:"none"`
	PingResolver             string               `name:"ping-resolver" help:"Resolver being used to resolver hostname to IP address during an ICMP ping or traceroute task" default:"172.20.0.53:53"`
	UpstreamJWTSecretFromEnv string               `name:"upstream-jwt-sec-env" help:"Name of the enviornment variable that stores the JWT token use to authenticate with the upstream ping events provider" default:"UPSTREAM_JWT_TOKEN"`
	UpstreamAPIPrefix        string               `name:"upstream-api-prefix" help:"The API prefix of the upstream server where to get ping events data" default:"https://ping2.sh/api"`

	// For authenticate itself to the hub
	JWTTokenFromEnvVar string `help:"The environment variable name to read the JWT token from" default:"JWT_TOKEN"`
	JWTTokenFromFile   string `help:"The file path to read the JWT token from" type:"path"`

	// For authenticate client's request
	JWTAuthSecretFromEnv  string `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret"`
	JWTAuthSecretFromFile string `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`

	// About IPRegistry.co
	IPRegistryCOAPIEndpoint     string `name:"ipregistry-api-endpoint" help:"APIEndpoint of IPRegistry.co" default:"https://ping2.sh/api/proxy/ipregistry"`
	IPRegistryCOAPIKeyEnv       string `name:"ipregistry-apikey-env" help:"Name of the environment variable that keep the APIKey for accessing ipregistry.co service" default:"IPREGISTRY_API_KEY"`
	IPRegistryAddBearerHeader   bool   `name:"ipregistry-add-bearer-header" help:"Append JWT token to IPRegistry API endpoint, this would be useful when the endpoint also require a 'Authorization: bearer <token>' header" default:"true"`
	IPRegistryCOAPIProviderName string `name:"ipregistry-provider-name" help:"Name for identifying the clearnet IPRegistry.co-compatible API service provider, useful especially in routing" default:"ipregistry"`

	DN42IP2LocationAPIEndpoint string `name:"dn42-ip2location-api-endpoint" help:"APIEndpoint of DN42 IP2Location provider, e.g. https://api.example.com/v1/ip2location , note that this has higher priority than DN42IPInfoProvider, so it would be used first once provided." default:"https://regquery.ping2.sh/ip2location/v1/query"`
}

func (cmd *MCPServerCmd) getJWTSecret() ([]byte, error) {
	return getJWTSecFromSomewhere(cmd.JWTAuthSecretFromEnv, cmd.JWTAuthSecretFromFile)
}

func (cmd *MCPServerCmd) getJWTToken() string {
	if envVar := cmd.JWTTokenFromEnvVar; envVar != "" {
		if token := os.Getenv(envVar); token != "" {
			return token
		}
	}

	if filePath := cmd.JWTTokenFromFile; filePath != "" {
		if data, err := os.ReadFile(filePath); err == nil {
			if token := strings.TrimSpace(string(data)); token != "" {
				return token
			}
		}
	}

	return ""
}

func (cmd *MCPServerCmd) registerAllAvailableIPInfoProviders(registry *pkgipinfo.IPInfoProviderRegistry) {
	// We are just trying to register all available IPInfo/GeoIP providers here.

	if apiendpoint := cmd.IPRegistryCOAPIEndpoint; apiendpoint != "" {
		log.Printf("Using IP2Location API Service: %s", apiendpoint)
		ipregistrycoAdapter := &pkgipinfo.IPRegistryAdapter{
			APIEndpoint: apiendpoint,
			Name:        cmd.IPRegistryCOAPIProviderName,
		}
		if cmd.IPRegistryAddBearerHeader {
			if ipregistrycoAdapter.AdditionalHeaders == nil {
				ipregistrycoAdapter.AdditionalHeaders = make(http.Header)
			}
			ipregistrycoAdapter.AdditionalHeaders.Set("Authorization", "bearer "+cmd.getJWTToken())
		}
		if envName := cmd.IPRegistryCOAPIKeyEnv; envName != "" {
			ipregistrycoAdapter.APIKey = os.Getenv(envName)
		}
		registry.RegisterAdapter(ipregistrycoAdapter)
	}

	var dn42IPInfoProvider pkgipinfo.GeneralIPInfoAdapter
	if apiEndpoint := cmd.DN42IP2LocationAPIEndpoint; apiEndpoint != "" {
		dn42IPInfoProvider = pkgipinfo.NewIP2LocationIPInfoAdapter(apiEndpoint, "", dn42IPInfoProviderName, nil)
		registry.RegisterAdapter(dn42IPInfoProvider)
	}
}

func (cmd *MCPServerCmd) Run(sharedCtx *pkgutils.GlobalSharedContext) error {

	mcpsrv := mcp.NewServer(&mcp.Implementation{
		Name:       cmd.MCPServerName,
		Title:      cmd.MCPServerTitle,
		WebsiteURL: cmd.MCPServerWebsiteURL,
	}, nil)

	ipinfoRegistry := pkgipinfo.NewIPInfoProviderRegistry()

	cmd.registerAllAvailableIPInfoProviders(ipinfoRegistry)

	ipqueryHandler := &pkgaihandlers.IPQueryHandler{
		IPInfoProviderRegistry: ipinfoRegistry,
	}
	ipqueryHandler.RegisterTool(mcpsrv)
	ipqueryHandler.RegisterResource(mcpsrv)

	pingEVProvider := &pkgbotdata.CloudPingEventsProvider{
		APIPrefix: cmd.UpstreamAPIPrefix,
		JWTToken:  os.Getenv(cmd.UpstreamJWTSecretFromEnv),
		Resolver:  cmd.PingResolver,
	}
	tracerouteHandler := &pkgaihandlers.TracerouteHandler{
		LocationsProvider:  pingEVProvider,
		PingEventsProvider: pingEVProvider,
	}
	tracerouteHandler.RegisterTool(mcpsrv)
	tracerouteHandler.RegisterResource(mcpsrv)

	pingHandler := &pkgaihandlers.PingHandler{
		LocationsProvider:  pingEVProvider,
		PingEventsProvider: pingEVProvider,
	}
	pingHandler.RegisterTool(mcpsrv)
	pingHandler.RegisterResource(mcpsrv)

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			// returning different mcp server depending on request to achieve multi-tenants and QoS.
			return mcpsrv
		},
		&mcp.StreamableHTTPOptions{Stateless: true},
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)

	var handler http.Handler = mux
	handler = pkghandler.WithLog(handler)
	if cmd.Authentication == AuthenticationMethodJWT {
		jwtSec, err := cmd.getJWTSecret()
		if err != nil {
			log.Panicf("failed to get JWT secret: %v", err)
		}
		keyProvider := pkgauth.NewStaticSecretProvider(jwtSec)
		jwtValidator := pkgauth.NewStaticKeyJWTValidator(keyProvider)
		handler = pkghandler.WithJWTAuth(handler, jwtValidator, nil)
	}
	handler = pkghandler.WithRealIP(handler)

	fmt.Printf("Starting HTTP server on %s\n", cmd.ListenAddress)
	return http.ListenAndServe(cmd.ListenAddress, handler)
}
