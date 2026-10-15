package cli

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkghandler "github.com/internetworklab/cloudping/pkg/handler"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
	"github.com/internetworklab/cloudping/pkg/webstatic"
	webassets "github.com/internetworklab/cloudping/web"
)

type WebAuthProxyCmd struct {
	JWTIssuer            string `help:"The issuer of the JWT token" default:"cloudping-web-proxy"`
	DefaultBackend       string `name:"default-backend" help:"The upstream base URL to reverse-proxy requests to, e.g. http://127.0.0.1:8091. When empty (default), the static web assets embedded into this binary are served directly."`
	LaunchTestingBackend string `name:"launch-testing-backend" help:"Specify the listen address to launch the backend of testing-purpose, for example: 127.0.0.1:8091. Leave it default (empty) to not launch it."`
	ListenAddress        string `name:"listen-address" help:"The listen address of the reverse proxy" default:":8092"`

	JWTAuthSecretFromEnv  string `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret"`
	JWTAuthSecretFromFile string `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`

	RedirectIfNoAuth string   `name:"redirect-if-no-auth" help:"The URL to redirect user to, when no valid authentication can be found from the request" default:"/login"`
	WhiteListPaths   []string `name:"add-white-list-path" help:"Additional white list paths, if the request path falls within these paths or any subpath of these paths, request will be passed to the backend directly."`
	RejectVisitor    bool     `name:"reject-visitor" help:"Reject requests from visitors (subjects with the 'visitor:' prefix)" default:"false"`

	VisitorSessionValidity      time.Duration `name:"validity-of-visitor-session" help:"Validity of visitor session" default:"168h"`
	VisitorSessionTicketGenIntv time.Duration `name:"visitor-jwt-ticket-gen-intv" help:"We issue visitor token based on some ticket generator, this is the interval of how fast it generate tickets" default:"1s"`

	LoginSessionValidity time.Duration `name:"named-session-validity" help:"Validity of named login session, such as those from Github or Google or whatever 3rd party SSO" default:"168h"`
	NonceValidity        time.Duration `name:"nonce-validity" help:"Validity of temporary usage random signed string" default:"5m"`

	GithubOAuthClientIdFromEnv  string `name:"github-oauth-client-id-from-env" help:"Name of the environment variable that contains the Github OAuth client ID" default:"GITHUB_OAUTH_CLIENT_ID"`
	GithubOAuthAppSecretFromEnv string `name:"github-oauth-app-secret-from-env" help:"Name of the environment variable that contains the Github OAuth app secret" default:"GITHUB_OAUTH_APP_SECRET"`
	GithubOAuthRedirURL         string `name:"github-oauth-redir-url" help:"The GitHub OAuth redirect URL"`

	GoogleOAuthClientIdFromEnv     string `name:"google-oauth-client-id-from-env" help:"Name of the environment variable that contains the Google OAuth client ID" default:"GOOGLE_OAUTH_CLIENT_ID"`
	GoogleOAuthClientSecretFromEnv string `name:"google-oauth-client-secret-from-env" help:"Name of the environment variable that contains the Google OAuth client secret" default:"GOOGLE_OAUTH_CLIENT_SECRET"`
	GoogleOAuthRedirURL            string `name:"google-oauth-redir-url" help:"The Google OAuth redirect URL"`

	IEdonOAuthClientIdFromEnv     string `name:"iedon-oauth-client-id-from-env" help:"Name of the environment variable that contains the iEdon OAuth client ID" default:"IEDON_OAUTH_CLIENT_ID"`
	IEdonOAuthClientSecretFromEnv string `name:"iedon-oauth-client-secret-from-env" help:"Name of the environment variable that contains the iEdon OAuth client secret" default:"IEDON_OAUTH_CLIENT_SECRET"`
	IEdonOAuthRedirURL            string `name:"iedon-oauth-redir-url" help:"The iEdon OAuth redirect URL"`

	KioubitOAuthClientIdFromEnv     string `name:"kioubit-oauth-client-id-from-env" help:"Name of the environment variable that contains the Kioubit OAuth client ID" default:"KIOUBIT_OAUTH_CLIENT_ID"`
	KioubitOAuthClientSecretFromEnv string `name:"kioubit-oauth-client-secret-from-env" help:"Name of the environment variable that contains the Kioubit OAuth client secret" default:"KIOUBIT_OAUTH_CLIENT_SECRET"`
	KioubitOAuthRedirURL            string `name:"kioubit-oauth-redir-url" help:"The Kioubit OAuth redirect URL"`

	EntraIDTenantIdFromEnv     string `name:"entra-id-tenant-id-from-env" help:"Name of the environment variable that contains the Microsoft Entra ID tenant ID" default:"ENTRA_ID_TENANT_ID"`
	EntraIDClientIdFromEnv     string `name:"entra-id-client-id-from-env" help:"Name of the environment variable that contains the Microsoft Entra ID client ID" default:"ENTRA_ID_CLIENT_ID"`
	EntraIDClientSecretFromEnv string `name:"entra-id-client-secret-from-env" help:"Name of the environment variable that contains the Microsoft Entra ID client secret" default:"ENTRA_ID_CLIENT_SECRET"`
	EntraIDRedirURL            string `name:"entra-id-redir-url" help:"The Microsoft Entra ID OAuth redirect URL"`
}

func (cmd *WebAuthProxyCmd) getJWTSecret() ([]byte, error) {
	return getJWTSecFromSomewhere(cmd.JWTAuthSecretFromEnv, cmd.JWTAuthSecretFromFile)
}

func (cmd *WebAuthProxyCmd) startTestingBackend(listenAddr string) {
	// Create a TCP listener on a random port (port 0 lets the OS pick one)
	if listenAddr == "" {
		listenAddr = ":0"
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Listener of testing-purpose backend started at %s", listener.Addr().String())

	// Set up the HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!\n")
		fmt.Fprintf(w, "URI: %s\n", r.RequestURI)
		for key, values := range r.Header {
			for _, value := range values {
				fmt.Fprintf(w, "%s: %s\n", key, value)
			}
		}
	})

	server := &http.Server{
		Handler: mux,
	}

	// Start serving in a goroutine so we can return the port
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Panic(err)
		}
	}()
}

func (cmd *WebAuthProxyCmd) startWebServer(ctx context.Context, sharedCtx *pkgutils.GlobalSharedContext) {

	jwtSec, err := cmd.getJWTSecret()
	if err != nil {
		log.Panicf("failed to get JWT secret: %v", err)
	}

	// todo: impl and use a dynm key provider
	keyProvider := pkgauth.NewStaticSecretProvider(jwtSec)

	jwtValidator := pkgauth.NewStaticKeyJWTValidator(keyProvider, pkgauth.NewNullBlackListProvider(), cmd.RejectVisitor)

	// By default the static web assets embedded into this binary are served;
	// when --default-backend is set, requests are reverse-proxied to that
	// upstream instead (useful for frontend development).
	var backendHandler http.Handler
	backendDesc := "embedded web assets"
	if cmd.DefaultBackend != "" {
		backendURL, err := url.Parse(cmd.DefaultBackend)
		if err != nil {
			log.Panic(fmt.Errorf("inproper backend url prefix %s: %w", cmd.DefaultBackend, err))
		}
		backendHandler = httputil.NewSingleHostReverseProxy(backendURL)
		backendDesc = fmt.Sprintf("reverse proxy to %s", cmd.DefaultBackend)
	} else {
		distFS, err := webassets.DistFS()
		if err != nil {
			log.Panic(fmt.Errorf("failed to access embedded web assets: %w", err))
		}
		if !webassets.HasDistAssets() {
			log.Printf("WARNING: no web assets embedded into this binary; " +
				"build the frontend first (cd web && npm ci && npm run build) before go build")
		}
		var versionFallback []byte
		if sharedCtx != nil && sharedCtx.BuildVersion != nil {
			versionFallback = sharedCtx.BuildVersion.Raw()
		}
		staticHandler, err := webstatic.NewHandler(distFS, webstatic.WithVersionTextFallback(versionFallback))
		if err != nil {
			log.Panic(fmt.Errorf("failed to create static web assets handler: %w", err))
		}
		backendHandler = staticHandler
	}
	backendHandler = pkghandler.WithLog(backendHandler)

	var proxyHandler http.Handler = backendHandler
	if cmd.RedirectIfNoAuth != "" {
		urlObj, err := url.Parse(cmd.RedirectIfNoAuth)
		if err != nil {
			log.Panicf("Invalid redirect url: %v", err)
		}

		redirTo := urlObj.String()

		proxyHandler = pkghandler.WithJWTAuth(proxyHandler, jwtValidator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("request intercepted: uri: %s, remote: %s", r.RequestURI, pkgutils.GetRemoteAddr(r))
			http.Redirect(w, r, redirTo, http.StatusTemporaryRedirect)
		}))
	}

	tokenIssuer := pkgauth.NewStaticKeyJWTIssuer(keyProvider, cmd.JWTIssuer)
	tickIssuer := pkgauth.NewSharedTickingTicketGenerator(cmd.VisitorSessionTicketGenIntv)
	tickIssuer.Run(ctx)
	visitorLoginHandler := &pkghandler.VisitorLoginHandler{
		Validity:        cmd.VisitorSessionValidity,
		JWTIssuer:       tokenIssuer,
		TicketGenerator: tickIssuer,
	}

	nonceIssuer := &pkgauth.StaticKeyNonceIssuer{
		NonceLifespan:  cmd.NonceValidity,
		SecretProvider: keyProvider,
	}
	githubLoginHandler := &pkghandler.GithubOAuthLoginHandler{
		NonceIssuer:          nonceIssuer,
		TokenIssuer:          tokenIssuer,
		SessionLifespan:      cmd.LoginSessionValidity,
		GithubOAuthClientId:  os.Getenv(cmd.GithubOAuthClientIdFromEnv),
		GithubOAuthAppSecret: os.Getenv(cmd.GithubOAuthAppSecretFromEnv),
		GithubOAuthRedirURL:  cmd.GithubOAuthRedirURL,
	}

	googleLoginHandler := &pkghandler.GoogleOAuthLoginHandler{
		NonceIssuer:             nonceIssuer,
		TokenIssuer:             tokenIssuer,
		SessionLifespan:         cmd.LoginSessionValidity,
		GoogleOAuthClientId:     os.Getenv(cmd.GoogleOAuthClientIdFromEnv),
		GoogleOAuthClientSecret: os.Getenv(cmd.GoogleOAuthClientSecretFromEnv),
		GoogleOAuthRedirURL:     cmd.GoogleOAuthRedirURL,
		LoginSuccessRedirectURL: "/",
	}

	muxer := http.NewServeMux()
	muxer.Handle("/", proxyHandler)
	muxer.Handle("/login/as/visitor", visitorLoginHandler)
	muxer.Handle("/login/as/github/", githubLoginHandler)
	muxer.Handle("/login/as/google/", googleLoginHandler)
	muxer.Handle("/login/exit", &pkghandler.LogoutHandler{})
	muxer.Handle("/login/profile", &pkghandler.BasicProfileHandler{
		TokenValidator: jwtValidator,
	})

	if iedonCLIId := os.Getenv(cmd.IEdonOAuthClientIdFromEnv); iedonCLIId != "" {
		if iedonCLISec := os.Getenv(cmd.IEdonOAuthClientSecretFromEnv); iedonCLISec != "" {
			iedonLoginHandler := &pkghandler.GenericOIDCLoginHandler{
				NonceIssuer:             nonceIssuer,
				TokenIssuer:             tokenIssuer,
				SessionLifespan:         cmd.LoginSessionValidity,
				LoginSuccessRedirectURL: "/",
				ProviderName:            "iedon",
				IssuerURL:               "https://auth.iedon.net",
				ClientId:                iedonCLIId,
				ClientSecret:            iedonCLISec,
				RedirectURL:             cmd.IEdonOAuthRedirURL,
			}
			muxer.Handle("/login/as/iedon/", iedonLoginHandler)
		}
	}

	if kioubitCLIId := os.Getenv(cmd.KioubitOAuthClientIdFromEnv); kioubitCLIId != "" {
		if kioubitCLISec := os.Getenv(cmd.KioubitOAuthClientSecretFromEnv); kioubitCLISec != "" {
			kioubitLoginHandler := &pkghandler.GenericOIDCLoginHandler{
				NonceIssuer:             nonceIssuer,
				TokenIssuer:             tokenIssuer,
				SessionLifespan:         cmd.LoginSessionValidity,
				LoginSuccessRedirectURL: "/",
				ProviderName:            "kioubit",
				IssuerURL:               "https://dn42.g-load.eu",
				ClientId:                kioubitCLIId,
				ClientSecret:            kioubitCLISec,
				RedirectURL:             cmd.KioubitOAuthRedirURL,
			}
			muxer.Handle("/login/as/kioubit/", kioubitLoginHandler)
		}
	}

	if entraTenantID := os.Getenv(cmd.EntraIDTenantIdFromEnv); entraTenantID != "" {
		if entraCLIId := os.Getenv(cmd.EntraIDClientIdFromEnv); entraCLIId != "" {
			if entraCLISec := os.Getenv(cmd.EntraIDClientSecretFromEnv); entraCLISec != "" {
				entraAuthority := fmt.Sprintf("https://login.microsoftonline.com/%s", entraTenantID)
				entraCred, err := confidential.NewCredFromSecret(entraCLISec)
				if err != nil {
					log.Panicf("failed to create Entra ID credential: %v", err)
				}
				entraMSALClient, err := confidential.New(entraAuthority, entraCLIId, entraCred)
				if err != nil {
					log.Panicf("failed to create Entra ID MSAL client: %v", err)
				}
				entraLoginHandler := &pkghandler.EntraIDLoginHandler{
					NonceIssuer:             nonceIssuer,
					TokenIssuer:             tokenIssuer,
					SessionLifespan:         cmd.LoginSessionValidity,
					LoginSuccessRedirectURL: "/",
					EntraIDTenantID:         entraTenantID,
					EntraIDClientID:         entraCLIId,
					EntraIDClientSecret:     entraCLISec,
					EntraIDRedirURL:         cmd.EntraIDRedirURL,
					MSALClient:              entraMSALClient,
				}
				muxer.Handle("/login/as/entra/", entraLoginHandler)
			}
		}
	}

	whiteList := make([]string, 0)
	whiteList = append(whiteList, cmd.WhiteListPaths...)

	log.Printf("White list: %s", strings.Join(whiteList, ", "))

	for _, path := range whiteList {
		muxer.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("request passthrough: uri: %s, remote: %s", r.RequestURI, pkgutils.GetRemoteAddr(r))
			backendHandler.ServeHTTP(w, r)
		}))
	}

	// Listen on a random TCP port
	listener, err := net.Listen("tcp", cmd.ListenAddress)
	if err != nil {
		log.Panic(err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	log.Printf("web server started at: %v, backend: %s", port, backendDesc)

	// Start serving in a goroutine with the log middleware chained on top of the proxy
	go func() {
		if err := http.Serve(listener, muxer); err != nil && err != http.ErrServerClosed {
			log.Panic(err)
		}
	}()
}

func (cmd *WebAuthProxyCmd) Run(sharedCtx *pkgutils.GlobalSharedContext) error {
	if cmd.LaunchTestingBackend != "" {
		cmd.startTestingBackend(cmd.LaunchTestingBackend)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd.startWebServer(ctx, sharedCtx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	log.Printf("Received %s, shutting down ...", sig.String())

	return nil
}
