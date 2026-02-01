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

	pkgconnreg "example.com/rbmq-demo/pkg/connreg"
	pkghandler "example.com/rbmq-demo/pkg/handler"
	pkgsafemap "example.com/rbmq-demo/pkg/safemap"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/gorilla/websocket"
	quicGo "github.com/quic-go/quic-go"
)

var upgrader = websocket.Upgrader{}

type HubCmd struct {
	PeerCAs       []string `help:"A list of path to the CAs use to verify peer certificates, can be specified multiple times"`
	Address       string   `help:"The address to listen on for private operations" default:":8080"`
	AddressPublic string   `help:"The address to listen on for public operations"`

	WebSocketPath string `help:"The path to the WebSocket endpoint" default:"/ws"`

	// When the hub is calling functions exposed by the agent, it have to authenticate itself to the agent.
	ClientCert    string `help:"The path to the client certificate" type:"path"`
	ClientCertKey string `help:"The path to the client certificate key" type:"path"`

	// Certificates to present to the clients when the hub itself is acting as a server.
	ServerCert    string `help:"The path to the server certificate" type:"path"`
	ServerCertKey string `help:"The path to the server certificate key" type:"path"`

	ResolverAddress         string `help:"The address of the resolver to use for DNS resolution" default:"172.20.0.53:53"`
	OutOfRespondRangePolicy string `help:"The policy to apply when a target is out of the respond range of a node" enum:"allow,deny" default:"allow"`

	MinPktInterval string `help:"The minimum interval between packets"`
	MaxPktTimeout  string `help:"The maximum timeout for a packet"`

	PktCountClamp *int `help:"The maximum number of packets to send for a single ping task"`

	WebSocketTimeout  string `help:"The timeout for a WebSocket connection" default:"60s"`
	QUICListenAddress string `help:"The address to listen on for QUIC" default:"0.0.0.0:18443"`

	JWTAuthListenAddress   string `name:"jwt-auth-listener-address" help:"Address to listen on for JWT authentication"`
	JWTAuthListenerCert    string `name:"jwt-auth-listener-cert" help:"Server TLS certificate"`
	JWTAuthListenerCertKey string `name:"jwt-auth-listener-cert-key" help:"Server TLS certificate key"`
	JWTAuthSecretFromEnv   string `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret"`
	JWTAuthSecretFromFile  string `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`
}

func getJWTSecret(hubCmd *HubCmd) ([]byte, error) {
	if envVar := hubCmd.JWTAuthSecretFromEnv; envVar != "" {
		secret := os.Getenv(envVar)
		if secret == "" {
			return nil, fmt.Errorf("JWT secret is not set in environment variable %s", envVar)
		}
		return []byte(secret), nil
	}

	if filePath := hubCmd.JWTAuthSecretFromFile; filePath != "" {
		secret, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read JWT secret file %s: %v", filePath, err)
		}
		if len(secret) == 0 {
			return nil, fmt.Errorf("JWT secret file %s is empty", filePath)
		}
		return secret, nil
	}

	return nil, fmt.Errorf("no JWT secret is set")
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

	customCAs, err := pkgutils.NewCustomCAPool(hubCmd.PeerCAs)
	if err != nil {
		log.Fatalf("Failed to create custom CA pool: %v", err)
	} else if len(hubCmd.PeerCAs) > 0 {
		log.Printf("Appended custom CAs: %s", strings.Join(hubCmd.PeerCAs, ", "))
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
		NextProtos:         []string{"h3", "ws", "h2", "http/1.1"},
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

	if hubCmd.QUICListenAddress != "" {
		quicListener, err := quicGo.ListenAddr(hubCmd.QUICListenAddress, privateServerSideTLSCfg, nil)
		if err != nil {
			log.Fatalf("Failed to listen on UDP address %s: %v", hubCmd.QUICListenAddress, err)
		}
		log.Printf("Listening on %s for QUIC operations", hubCmd.QUICListenAddress)
		quicHandler := pkghandler.QUICHandler{
			Cr:       cr,
			Timeout:  wsTimeout,
			Listener: quicListener,
		}
		go quicHandler.Serve()
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

	// muxerPrivate is for privileged rw operations
	muxerPrivate := http.NewServeMux()
	muxerPrivate.Handle(hubCmd.WebSocketPath, wsHandler)

	// muxerPublic is for public low-privileged operations
	muxerPublic := http.NewServeMux()
	muxerPublic.Handle("/conns", connsHandler)
	muxerPublic.Handle("/ping", pingHandler)
	muxerPublic.Handle("/version", pkghandler.NewVersionHandler(sharedCtx))

	if hubCmd.Address != "" {
		privateListener, err := tls.Listen("tcp", hubCmd.Address, privateServerSideTLSCfg)
		if err != nil {
			log.Fatalf("Failed to listen on address %s: %v", hubCmd.Address, err)
		}
		log.Printf("Listening on %s for private operations", hubCmd.Address)

		go func() {
			log.Printf("Starting private server on %s", privateListener.Addr())
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

	if hubCmd.AddressPublic != "" {
		publicListener, err := net.Listen("tcp", hubCmd.AddressPublic)
		if err != nil {
			log.Fatalf("Failed to listen on address %s: %v", hubCmd.AddressPublic, err)
		}
		log.Printf("Listening on %s for public operations", hubCmd.AddressPublic)
		go func() {
			log.Printf("Starting public server on %s", publicListener.Addr())
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

	if listenAddress := hubCmd.JWTAuthListenAddress; listenAddress != "" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			NextProtos:         []string{"h3", "ws", "h2", "http/1.1"},
		}
		if certPath := hubCmd.JWTAuthListenerCert; certPath != "" {
			if keyPath := hubCmd.JWTAuthListenerCertKey; keyPath != "" {
				cert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err != nil {
					log.Fatalf("Failed to load server certificate for JWT listener: %v", err)
				}
				tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
				log.Printf("Loaded server cert pair from %s and %s for JWT listener", certPath, keyPath)
			}
		}

		quicListener, err := quicGo.ListenAddr(listenAddress, tlsConfig, nil)
		if err != nil {
			log.Fatalf("Failed to listen on UDP address %s: %v", listenAddress, err)
		}

		jwtSec, err := getJWTSecret(&hubCmd)
		if err != nil {
			log.Fatalf("Failed to load JWT secret: %v", err)
		}
		quicHandler := pkghandler.QUICHandler{
			Cr:        cr,
			Timeout:   wsTimeout,
			Listener:  quicListener,
			JWTSecret: jwtSec,
		}
		go quicHandler.Serve()
		log.Printf("Listening on UDP address %s for JWT-authenticated handlers", quicListener.Addr())
	}

	sig := <-sigs
	log.Printf("Received %s, shutting down ...", sig.String())
	sm.Close()

	return nil
}
