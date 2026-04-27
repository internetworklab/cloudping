package cli

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkghandler "github.com/internetworklab/cloudping/pkg/handler"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

type WebAuthProxyCmd struct {
	DefaultBackend       string `name:"default-backend" help:"The host:port pair of the default backend" default:"127.0.0.1:8091"`
	LaunchTestingBackend string `name:"launch-testing-backend" help:"Specify the listen address to launch the backend of testing-purpose, for example: 127.0.0.1:8091. Leave it default (empty) to not launch it."`
	ListenAddress        string `name:"listen-address" help:"The listen address of the reverse proxy" default:":8092"`

	JWTAuthSecretFromEnv  string `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret"`
	JWTAuthSecretFromFile string `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`

	RedirectIfNoAuth string `name:"redirect-if-no-auth" help:"The URL to redirect user to, when no valid authentication can be found from the request" default:"/login"`
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

func (cmd *WebAuthProxyCmd) startReverseProxy(backendURLPrefix string) {
	// Parse the backend URL
	backendURL, err := url.Parse(backendURLPrefix)
	if err != nil {
		log.Panic(fmt.Errorf("inproper backend url prefix %s: %w", backendURL, err))
	}

	// Create a reverse proxy that forwards requests to the backend
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	var proxyHandler http.Handler = proxy
	proxyHandler = pkghandler.WithLog(proxyHandler)
	if cmd.RedirectIfNoAuth != "" {
		urlObj, err := url.Parse(cmd.RedirectIfNoAuth)
		if err != nil {
			log.Panicf("Invalid redirect url: %v", err)
		}

		redirPath := urlObj.Path
		redirTo := urlObj.String()

		jwtSec, err := cmd.getJWTSecret()
		if err != nil {
			log.Panicf("failed to get JWT secret: %v", err)
		}

		// todo: impl and use a dynm key provider
		keyProvider := pkgauth.NewStaticSecretProvider(jwtSec)

		jwtValidator := pkgauth.NewStaticKeyJWTValidator(keyProvider)
		unchained := proxyHandler
		proxyHandler = pkghandler.WithJWTAuth(proxyHandler, jwtValidator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent redirect loop: if the request is already for the redirect target,
			// let it pass through to the backend instead of redirecting again.
			if r.URL.Path == redirPath {
				unchained.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, redirTo, http.StatusTemporaryRedirect)
		}))
	}

	// Listen on a random TCP port
	listener, err := net.Listen("tcp", cmd.ListenAddress)
	if err != nil {
		log.Panic(err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	log.Printf("reverse proxy started at: %v, forwarding to: %v", port, backendURLPrefix)

	// Start serving in a goroutine with the log middleware chained on top of the proxy
	go func() {
		if err := http.Serve(listener, proxyHandler); err != nil && err != http.ErrServerClosed {
			log.Panic(err)
		}
	}()
}

func (cmd *WebAuthProxyCmd) Run(sharedCtx *pkgutils.GlobalSharedContext) error {
	cmd.startTestingBackend(cmd.LaunchTestingBackend)

	cmd.startReverseProxy(cmd.DefaultBackend)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	log.Printf("Received %s, shutting down ...", sig.String())

	return nil
}
