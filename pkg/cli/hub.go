package cli

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pkgauth "example.com/rbmq-demo/pkg/auth"
	pkgconnreg "example.com/rbmq-demo/pkg/connreg"
	pkghandler "example.com/rbmq-demo/pkg/handler"
	pkgproxy "example.com/rbmq-demo/pkg/proxy"
	pkgsafemap "example.com/rbmq-demo/pkg/safemap"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/gorilla/websocket"
	quicGo "github.com/quic-go/quic-go"
)

var upgrader = websocket.Upgrader{}

type HubCmd struct {
	PublicHTTPListenAddress  string `name:"public-http-listen-address" help:"The address to listen on for public operations"`
	WebSocketListenAddress   string `name:"mtls-websocket-listen-address" help:"The address to listen on for private operations"`
	QUICMTLSListenAddress    string `name:"mtls-quic-listen-address" help:"The address to listen on for QUIC"`
	QUICJWTAuthListenAddress string `name:"jwt-quic-listen-address" help:"Address to listen on for JWT authentication"`

	WebSocketPath    string `help:"The path to the WebSocket upgrade handler" default:"/ws"`
	WebSocketTimeout string `help:"The timeout for a WebSocket connection" default:"60s"`

	// When the hub is calling functions exposed by the agent, it have to authenticate itself to the agent.
	ClientCert    string `help:"The path to the client certificate" type:"path"`
	ClientCertKey string `help:"The path to the client certificate key" type:"path"`

	// Certificates to present to the clients when the hub itself is acting as a server.
	PeerCA        []string `name:"peer-ca" help:"A list of path to the CAs use to verify peer certificates, can be specified multiple times"`
	ServerCert    string   `help:"The path to the server certificate" type:"path"`
	ServerCertKey string   `help:"The path to the server certificate key" type:"path"`

	ResolverAddress         string `help:"The address of the resolver to use for DNS resolution" default:"172.20.0.53:53"`
	OutOfRespondRangePolicy string `help:"The policy to apply when a target is out of the respond range of a node" enum:"allow,deny" default:"allow"`
	MinPktInterval          string `help:"The minimum interval between packets"`
	MaxPktTimeout           string `help:"The maximum timeout for a packet"`
	PktCountClamp           *int   `help:"The maximum number of packets to send for a single ping task"`

	JWTAuthSecretFromEnv  string `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret"`
	JWTAuthSecretFromFile string `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`
}

func (hubCmd *HubCmd) getJWTSecret() ([]byte, error) {
	return getJWTSecFromSomewhere(hubCmd.JWTAuthSecretFromEnv, hubCmd.JWTAuthSecretFromFile)
}

const defaultWebSocketTimeout = 60 * time.Second

func (hubCmd HubCmd) Run(sharedCtx *pkgutils.GlobalSharedContext) error {
	var minPktInterval *time.Duration
	var maxPktTimeout *time.Duration

	if hubCmd.MinPktInterval != "" {
		intv, err := time.ParseDuration(hubCmd.MinPktInterval)
		if err != nil {
			return fmt.Errorf("failed to parse min packet interval: %v", err)
		}
		log.Printf("Parsed min packet interval: %s", intv.String())
		minPktInterval = &intv
	}
	if hubCmd.MaxPktTimeout != "" {
		tmt, err := time.ParseDuration(hubCmd.MaxPktTimeout)
		if err != nil {
			return fmt.Errorf("failed to parse max packet timeout: %v", err)
		}
		log.Printf("Parsed max packet timeout: %s", tmt.String())
		maxPktTimeout = &tmt
	}

	if hubCmd.PktCountClamp != nil {
		log.Printf("PktCountClamp is set to %d", *hubCmd.PktCountClamp)
	}

	customCAs, err := pkgutils.NewCustomCAPool(hubCmd.PeerCA)
	if err != nil {
		log.Fatalf("Failed to create custom CA pool: %v", err)
	} else if len(hubCmd.PeerCA) > 0 {
		log.Printf("Appended custom CAs: %s", strings.Join(hubCmd.PeerCA, ", "))
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sm := pkgsafemap.NewSafeMap()
	cr := pkgconnreg.NewConnRegistry(sm)

	wsTimeout := defaultWebSocketTimeout
	if timeout, err := time.ParseDuration(hubCmd.WebSocketTimeout); err == nil && int64(timeout) >= 0 {
		wsTimeout = timeout
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		log.Fatalf("Failed to get system cert pool: %v", err)
	}
	// TLSConfig when functioning as a server (i.e. we are the server, while the peer is the client)
	privateServerSideTLSCfg := &tls.Config{
		ClientAuth:         tls.RequireAndVerifyClientCert,
		ClientCAs:          certPool,
		InsecureSkipVerify: false,
		NextProtos:         []string{"h3"},
	}
	if hubCmd.ServerCert != "" && hubCmd.ServerCertKey != "" {
		cert, err := tls.LoadX509KeyPair(hubCmd.ServerCert, hubCmd.ServerCertKey)
		if err != nil {
			log.Fatalf("Failed to load server certificate: %v", err)
		}
		if privateServerSideTLSCfg.Certificates == nil {
			privateServerSideTLSCfg.Certificates = make([]tls.Certificate, 0)
		}
		privateServerSideTLSCfg.Certificates = append(privateServerSideTLSCfg.Certificates, cert)
		log.Printf("Loaded server certificate: %s and key: %s", hubCmd.ServerCert, hubCmd.ServerCertKey)
	}
	if customCAs != nil {
		privateServerSideTLSCfg.ClientCAs = customCAs
	}

	if listenAddress := hubCmd.QUICMTLSListenAddress; listenAddress != "" {
		quicListener, err := quicGo.ListenAddr(listenAddress, privateServerSideTLSCfg, nil)
		if err != nil {
			log.Fatalf("Failed to listen on UDP address %s: %v", listenAddress, err)
		}
		quicHandler := pkghandler.QUICHandler{
			Cr:                cr,
			Timeout:           wsTimeout,
			Listener:          quicListener,
			ShouldValidateJWT: false,
		}
		go quicHandler.Serve()
		log.Printf("Listening and serving on %s for mtls-authenticated QUIC operations", listenAddress)
	}

	wsHandler := pkghandler.NewWebsocketHandler(&upgrader, cr, wsTimeout)
	connsHandler := pkghandler.NewConnsHandler(cr)
	var clientTLSConfig *tls.Config = &tls.Config{}
	if customCAs != nil {
		clientTLSConfig.RootCAs = customCAs
	}
	if hubCmd.ClientCert != "" && hubCmd.ClientCertKey != "" {
		cert, err := tls.LoadX509KeyPair(hubCmd.ClientCert, hubCmd.ClientCertKey)
		if err != nil {
			log.Fatalf("Failed to load client certificate: %v", err)
		}
		if clientTLSConfig.Certificates == nil {
			clientTLSConfig.Certificates = make([]tls.Certificate, 0)
		}
		clientTLSConfig.Certificates = append(clientTLSConfig.Certificates, cert)
		log.Printf("Loaded client certificate: %s and key: %s", hubCmd.ClientCert, hubCmd.ClientCertKey)
	}
	resolver := pkgutils.NewCustomResolver(&hubCmd.ResolverAddress, 10*time.Second)
	pingHandler := &pkghandler.PingTaskHandler{
		ConnRegistry:            cr,
		ClientTLSConfig:         clientTLSConfig,
		Resolver:                resolver,
		OutOfRespondRangePolicy: pkghandler.OutOfRespondRangePolicy(hubCmd.OutOfRespondRangePolicy),
		MinPktInterval:          minPktInterval,
		MaxPktTimeout:           maxPktTimeout,
		PktCountClamp:           hubCmd.PktCountClamp,
	}

	jwtSec, err := hubCmd.getJWTSecret()
	if err != nil {
		return fmt.Errorf("failed to get JWT secret: %v", err)
	}

	var proxyHandler http.Handler = &pkgproxy.IP2LocationProxyHandler{
		BackendEndpoint: "https://api.ip2location.io/v2/",
		APIKey:          os.Getenv("IP2LOCATION_API_KEY"),
	}
	proxyHandler = pkgauth.WithJWTAuth(proxyHandler, jwtSec)

	// muxerPrivate is for privileged rw operations
	muxerPrivate := http.NewServeMux()
	muxerPrivate.Handle(hubCmd.WebSocketPath, wsHandler)

	// muxerPublic is for public low-privileged operations
	muxerPublic := http.NewServeMux()
	muxerPublic.Handle("/conns", connsHandler)
	muxerPublic.Handle("/ping", pingHandler)
	muxerPublic.Handle("/version", pkghandler.NewVersionHandler(sharedCtx))
	muxerPublic.Handle("/proxy/ip2location", proxyHandler)

	if listenAddress := hubCmd.WebSocketListenAddress; listenAddress != "" {
		privateListener, err := tls.Listen("tcp", listenAddress, privateServerSideTLSCfg)
		if err != nil {
			log.Fatalf("Failed to listen on address %s: %v", listenAddress, err)
		}

		go func() {
			log.Printf("Listening and serving on %s for private operations from mtls-authenticated websocket", listenAddress)
			privateServer := http.Server{
				Handler: muxerPrivate,
			}
			err = privateServer.Serve(privateListener)
			if err != nil {
				if err != http.ErrServerClosed {
					log.Fatalf("Failed to serve: %v", err)
				}
			}
		}()

	}

	if listenAddress := hubCmd.PublicHTTPListenAddress; listenAddress != "" {
		publicListener, err := net.Listen("tcp", listenAddress)
		if err != nil {
			log.Fatalf("Failed to listen on address %s: %v", listenAddress, err)
		}
		go func() {
			log.Printf("Listening and serving on %s for public operations", listenAddress)
			publicServer := http.Server{
				Handler: muxerPublic,
			}
			err = publicServer.Serve(publicListener)
			if err != nil {
				if err != http.ErrServerClosed {
					log.Fatalf("Failed to serve: %v", err)
				}
			}
		}()

	}

	if listenAddress := hubCmd.QUICJWTAuthListenAddress; listenAddress != "" {
		tlsConfig := privateServerSideTLSCfg.Clone()
		tlsConfig.ClientAuth = tls.NoClientCert

		quicListener, err := quicGo.ListenAddr(listenAddress, tlsConfig, nil)
		if err != nil {
			log.Fatalf("Failed to listen on UDP address %s: %v", listenAddress, err)
		}

		quicHandler := pkghandler.QUICHandler{
			Cr:                cr,
			Timeout:           wsTimeout,
			Listener:          quicListener,
			JWTSecret:         jwtSec,
			ShouldValidateJWT: true,
		}
		go quicHandler.Serve()
		log.Printf("Listening and serving on UDP address %s for JWT-authenticated handlers", quicListener.Addr())
	}

	sig := <-sigs
	log.Printf("Received %s, shutting down ...", sig.String())
	sm.Close()

	return nil
}
