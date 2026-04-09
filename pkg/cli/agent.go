package cli

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	pkgconnreg "github.com/internetworklab/cloudping/pkg/connreg"
	pkghandler "github.com/internetworklab/cloudping/pkg/handler"
	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	pkgmyprom "github.com/internetworklab/cloudping/pkg/myprom"
	pkgnodereg "github.com/internetworklab/cloudping/pkg/nodereg"
	pkgratelimit "github.com/internetworklab/cloudping/pkg/ratelimit"
	pkgrouting "github.com/internetworklab/cloudping/pkg/routing"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type AgentCmd struct {
	NodeName            string `help:"Nodename to advertise to the hub, leave it empty for not advertising itself to the hub"`
	HttpEndpoint        string `help:"HTTP endpoint to advertise to the hub"`
	ExactLocationLatLon string `help:"The exact geographic location to advertise to the hub, when present. Format: <latitude>,<longitude>"`
	CountryCode         string `help:"The country code to advertise to the hub, when present. Format: <iso3166-alpha2-country-code>"`
	CityName            string `help:"The city name to advertise to the hub, when present. Format: <name-of-the-city>"`
	ASN                 string `help:"The ASN of the ISP that provides internet connectivity to the node. Format: AS<number>, e.g. AS65001"`
	ISP                 string `help:"The name of the ISP that provides internet connectivity to the node"`
	DN42ASN             string `name:"dn42-asn" help:"The ASN of the ISP that provides DN42 connectivity to the node. Format: AS<number>, e.g. AS4242421234"`
	DN42ISP             string `name:"dn42-isp" help:"The name of the ISP that provides DN42 connectivity to the node"`

	// If both the (ws) server address and the QUIC server address are empty, it won't register itself to the hub.
	ServerAddress     string `help:"WebSocket endpoint of the hub"`
	QUICServerAddress string `help:"QUIC endpoint of the hub" default:"globalping-hub.exploro.one:18448"`

	// PeerCA are use to verify certs presented by the peer,
	// For agent, the peer is the hub, for hub, the peer is the agent.
	// Simply put, PeerCA are what agent is use to verify the hub's cert, and what hub is use to verify the agent's cert.
	PeerCA []string `name:"peer-ca" help:"PeerCAs are custom CAs use to verify the hub (server)'s certificate, when none are provided, system CAs are used to do the job. PeerCAs are also use to verify the client's certificate when functioning as a server." default:"https://github.com/internetworklab/cloudping/raw/refs/heads/master/confed/hub/ca.pem"`

	// Agent will connect to the hub (sometimes), so this is the TLS name (mostly CN field or DNS Alt Name) of the hub.
	ServerName string `help:"We connect to the server via TLS, this is to verify the server's certificate, at least one of the DNSAltName fields in the server-presented certificate must match this value"`

	AgentTickInterval string `help:"The interval between node registration agent's tick" default:"5s"`

	JWTTokenFromEnvVar string `help:"The environment variable name to read the JWT token from" default:"JWT_TOKEN"`
	JWTTokenFromFile   string `help:"The file path to read the JWT token from" type:"path"`

	// When the agent is connecting to the hub, the hub needs to authenticate the client, so the client (the agent) also have to present a cert
	// to complete the m-TLS authentication process.
	ClientCert    string `help:"The path to the client certificate, i.e. the cert to use when acting as a client" type:"path"`
	ClientCertKey string `help:"The path to the client certificate key, i.e. the key of the cert to use when acting as a client" type:"path"`

	// Agent also functions as a server (i.e. provides public tls-secured endpoint, so it might also needs a cert pair)
	ServerCert    string `help:"The path to the server certificate, i.e. the cert to use when acting as a server" type:"path"`
	ServerCertKey string `help:"The path to the server key, i.e. the key of the cert to use when acting as a server" type:"path"`

	TLSListenAddress string `help:"Address to listen on for incoming TLS connections when the hub is expected to call this via the advertised public TLS endpoint"`

	// when http listen address is not empty, it will serve http requests without any TLS authentication
	HTTPListenAddress string `help:"Address to listen on for plaintext HTTP requests, only use this for debugging purposes"`

	// IPInfo/IP2Location related settings
	DN42IPInfoProvider             string `name:"dn42-ipinfo-provider" help:"APIEndpoint of DN42 IPInfo provider, e.g. https://api.example.com/v1/ipinfo"`
	DN42IP2LocationAPIEndpoint     string `name:"dn42-ip2location-api-endpoint" help:"APIEndpoint of DN42 IP2Location provider, e.g. https://api.example.com/v1/ip2location , note that this has higher priority than DN42IPInfoProvider" default:"https://regquery.ping2.sh/ip2location/v1/query"`
	IPInfoCacheValiditySecs        int    `name:"ipinfo-cache-validity-secs" help:"The validity of the IPInfo cache in seconds" default:"600"`
	IP2LocationAPIEndpoint         string `name:"ip2location-api-endpoint" help:"APIEndpoint of IP2Location IPInfo provider" default:"https://ping2.sh/api/proxy/ip2location"`
	AppendJWTTokenToIP2LocationAPI bool   `name:"append-bearer-header-to-ip2location-requests" help:"Append JWT token to IP2Location API endpoint" default:"true"`

	// Prometheus stuffs
	MetricsListenAddress string `help:"Address of the listener for exposing prometheus metrics, e.g. :2112"`
	MetricsPath          string `help:"Path of the Prometheus metrics endpoint, e.g. /metrics" default:"/metrics"`

	// Probing capabilities declaration
	SupportUDP            bool     `help:"Declare supportness for UDP traceroute" default:"true"`
	SupportPMTU           bool     `help:"Declare supportness for PMTU discovery" default:"true"`
	SupportTCP            bool     `help:"Declare supportness for TCP-flavored ping" default:"true"`
	SupportDNS            bool     `help:"Declare supportness for DNS probing" default:"true"`
	SupportHTTP           bool     `name:"support-http" help:"Declare supportness for HTTP probing" default:"true"`
	HTTPProbeAdditionalCA []string `name:"http-probe-add-ca" help:"CAs to trust in addition to the systems' default CA store when doing DNS probe (DoT) or HTTP probe"`

	// Some Debugging features
	LogEchoReplies bool `help:"Log echo replies" default:"false"`

	// Throttling/restriction related settings for how to protect ourselves from abuses
	SharedOutboundRateLimit                int      `name:"shared-outbound-ratelimit" help:"Shared quota for limiting the outbound traffic (packets per refresh interval)" default:"100"`
	SharedOutboundRateLimitRefreshInterval string   `name:"shared-outbound-ratelimit-refresh-interval" help:"The refresh interval of the shared outbound rate limit" default:"1s"`
	RespondRange                           []string `help:"A list of CIDR ranges defining what queries this agent will respond to, by default, all queries will be responded."`
	DomainRespondRange                     []string `help:"A domain respond range, when present, is a list of domain patterns that defines what queries will be responded in terms of domain name."`
}

func (agentCmd *AgentCmd) getJWTToken() string {
	if envVar := agentCmd.JWTTokenFromEnvVar; envVar != "" {
		if token := os.Getenv(envVar); token != "" {
			return token
		}
	}

	if filePath := agentCmd.JWTTokenFromFile; filePath != "" {
		if data, err := os.ReadFile(filePath); err == nil {
			if token := strings.TrimSpace(string(data)); token != "" {
				return token
			}
		}
	}

	return ""
}

func getClearnetIPInfoAdapter(agentCmd *AgentCmd) (pkgipinfo.GeneralIPInfoAdapter, error) {
	ip2LocationEndpoint := agentCmd.IP2LocationAPIEndpoint
	if ip2LocationEndpoint != "" {
		ip2LocationAPIKey := os.Getenv("IP2LOCATION_API_KEY")
		log.Printf("Using IP2Location API Service: %s", ip2LocationEndpoint)
		var extraHeaders http.Header = nil
		if agentCmd.AppendJWTTokenToIP2LocationAPI {
			extraHeaders = http.Header{}
			extraHeaders.Set("Authorization", fmt.Sprintf("Bearer %s", agentCmd.getJWTToken()))
		}
		ip2LocationIPInfoAdapter := pkgipinfo.NewIP2LocationIPInfoAdapter(ip2LocationEndpoint, ip2LocationAPIKey, "ip2location", extraHeaders)
		return ip2LocationIPInfoAdapter, nil
	}

	ipinfoLiteToken := os.Getenv("IPINFO_TOKEN")
	if ipinfoLiteToken != "" {
		log.Printf("Using IPInfo Lite API Service: %s", ipinfoLiteToken)
		ipinfoLiteIPInfoAdapter, err := pkgipinfo.NewIPInfoAdapter(&ipinfoLiteToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create IPInfo Lite adapter: %v", err)
		}
		return ipinfoLiteIPInfoAdapter, nil
	}

	return nil, fmt.Errorf("no valid ipinfo provider found")
}

func getDN42IPInfoAdapter(agentCmd *AgentCmd) (pkgipinfo.GeneralIPInfoAdapter, error) {
	if agentCmd.DN42IP2LocationAPIEndpoint != "" {
		return pkgipinfo.NewIP2LocationIPInfoAdapter(agentCmd.DN42IP2LocationAPIEndpoint, "", "dn42", nil), nil
	}

	return pkgipinfo.NewDN42IPInfoAdapter(agentCmd.DN42IPInfoProvider), nil
}

const minTickInterval = 1000 * time.Millisecond

func (agentCmd *AgentCmd) Run(sharedCtx *pkgutils.GlobalSharedContext) error {

	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rlRefreshIntv, err := time.ParseDuration(agentCmd.SharedOutboundRateLimitRefreshInterval)
	if err != nil {
		log.Fatalf("failed to parse shared outbound rate limit refresh interval: %v", err)
	}

	sharedRateLimitPool := &pkgratelimit.MemoryBasedRateLimitPool{
		RefreshIntv:     rlRefreshIntv,
		NumTokensPerKey: agentCmd.SharedOutboundRateLimit,
	}
	sharedRateLimitPool.Run(ctx)

	sharedRateLimitEnforcer := &pkgratelimit.MemoryBasedRateLimiter{
		Pool:   sharedRateLimitPool,
		GetKey: pkgratelimit.GlobalKeyFunc,
	}

	log.Printf("Using bucket-based token rate limiter, with %d tokens per slot, refreshing every %s", agentCmd.SharedOutboundRateLimit, rlRefreshIntv.String())

	counterStore := pkgmyprom.NewCounterStore()
	ctx = context.WithValue(ctx, pkgutils.CtxKeyPrometheusCounterStore, counterStore)

	counterStore.StartedTime.Set(float64(time.Now().Unix()))

	ipinfoReg := pkgipinfo.NewIPInfoProviderRegistry()

	classicIPInfoAdapter, err := getClearnetIPInfoAdapter(agentCmd)
	if err != nil {
		log.Fatalf("failed to initialize IPInfo adapter: %v", err)
	}
	// skip registering named ipinfo providers to the registry,
	// to prevent users from intentionally bypassing the (cached) auto ipinfo dispatcher.
	// ipinfoReg.RegisterAdapter(classicIPInfoAdapter)
	dn42IPInfoAdapter, err := getDN42IPInfoAdapter(agentCmd)
	if err != nil {
		log.Fatalf("failed to initialize DN42 IPInfo adapter: %v", err)
	}
	// ipinfoReg.RegisterAdapter(dn42IPInfoAdapter)
	randomIPInfoAdapter := pkgipinfo.NewRandomIPInfoAdapter()
	ipinfoReg.RegisterAdapter(randomIPInfoAdapter)

	autoIPInfoDispatcher := &pkgipinfo.AutoIPInfoDispatcher{
		Router: pkgrouting.NewSimpleRouter(),
	}
	autoIPInfoDispatcher.SetUpDefaultRoutes(dn42IPInfoAdapter, classicIPInfoAdapter)

	ipinfoCacheHook := func(ctx context.Context, stats pkgipinfo.IPInfoRequestStats) {
		// will remove this logging code later

		commonLabels := ctx.Value(pkgutils.CtxKeyPromCommonLabels).(prometheus.Labels)
		counterStore.IPInfoServedDurationMs.With(commonLabels).Add(stats.DurationMs)
		ipinfoRequestLabels := maps.Clone(commonLabels)
		ipinfoRequestLabels[pkgmyprom.PromLabelCacheHit] = strconv.FormatBool(stats.CacheHit)
		ipinfoRequestLabels[pkgmyprom.PromLabelHasError] = strconv.FormatBool(stats.HasError)
		counterStore.IPInfoRequests.With(ipinfoRequestLabels).Add(1.0)
	}
	ipinfoCacheValidity := time.Duration(agentCmd.IPInfoCacheValiditySecs) * time.Second
	log.Printf("IPInfo cache validity: %s", ipinfoCacheValidity.String())
	cachedAutoIPInfoDispatcher := pkgipinfo.NewCacheIPInfoProvider(autoIPInfoDispatcher, ipinfoCacheValidity, ipinfoCacheHook)
	cachedAutoIPInfoDispatcher.Run(ctx)
	ipinfoReg.RegisterAdapter(cachedAutoIPInfoDispatcher)

	customCAs, err := pkgutils.NewCustomCAPool(agentCmd.PeerCA)
	if err != nil {
		log.Fatalf("Failed to load custom CA pool: %v", err)
	}

	respondRangeNet := make([]net.IPNet, 0)
	for _, rangeStr := range agentCmd.RespondRange {
		_, nw, err := net.ParseCIDR(rangeStr)
		if err != nil {
			log.Fatalf("failed to parse respond range %s: %v", rangeStr, err)
		}
		respondRangeNet = append(respondRangeNet, *nw)
	}

	domaonRespondRange := make([]regexp.Regexp, 0)
	for _, domainPattern := range agentCmd.DomainRespondRange {
		domainRegexp, err := regexp.Compile(domainPattern)
		if err != nil {
			log.Fatalf("failed to compile domain pattern %s: %v", domainPattern, err)
		}
		domaonRespondRange = append(domaonRespondRange, *domainRegexp)
	}

	handler := &pkghandler.PingHandler{
		IPInfoReg:             ipinfoReg,
		RespondRange:          respondRangeNet,
		DomainRespondRange:    domaonRespondRange,
		HTTPProbeAdditionalCA: agentCmd.HTTPProbeAdditionalCA,
	}

	muxer := http.NewServeMux()
	muxer.Handle("/simpleping", handler)
	muxer.Handle("/tcping", handler)
	muxer.Handle("/dnsprobe", handler)
	muxer.Handle("/version", pkghandler.NewVersionHandler(sharedCtx))

	var muxedHandler http.Handler = muxer
	muxedHandler = pkgmyprom.WithCounterStoreHandler(muxedHandler, counterStore)
	muxedHandler = pkgratelimit.WithRatelimiters(muxedHandler, sharedRateLimitEnforcer)

	if promListenAddr := agentCmd.MetricsListenAddress; promListenAddr != "" {
		prometheusListener, err := net.Listen("tcp", promListenAddr)
		if err != nil {
			log.Fatalf("failed to listen on address for prometheus metrics: %s: %v", promListenAddr, err)
		}
		defer prometheusListener.Close()
		log.Printf("Listening on address %s for prometheus metrics", promListenAddr)

		go func() {
			log.Printf("Serving prometheus metrics on address %s", prometheusListener.Addr())
			handler := promhttp.Handler()
			serveMux := http.NewServeMux()
			serveMux.Handle(agentCmd.MetricsPath, handler)
			server := http.Server{
				Handler: serveMux,
			}
			if err := server.Serve(prometheusListener); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.Fatalf("failed to serve prometheus metrics: %v", err)
				}
				log.Println("Prometheus metrics server exitted")
			}
		}()
	}

	if tlsListenAddr := agentCmd.TLSListenAddress; tlsListenAddr != "" {
		// TLSConfig to apply when acting as a server (i.e. we provide services, peer calls us)
		serverSideTLSCfg := &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
		}
		if customCAs != nil {
			serverSideTLSCfg.ClientCAs = customCAs
		}

		if srvCert := agentCmd.ServerCert; srvCert != "" {
			if srvCertKey := agentCmd.ServerCertKey; srvCertKey != "" {
				cert, err := tls.LoadX509KeyPair(srvCert, srvCertKey)
				if err != nil {
					log.Fatalf("failed to load server certificate: %v", err)
				}
				if serverSideTLSCfg.Certificates == nil {
					serverSideTLSCfg.Certificates = make([]tls.Certificate, 0)
				}
				serverSideTLSCfg.Certificates = append(serverSideTLSCfg.Certificates, cert)
				log.Printf("Loaded server certificate: %s and key: %s", srvCert, srvCertKey)
			}
		}

		listener, err := tls.Listen("tcp", tlsListenAddr, serverSideTLSCfg)
		if err != nil {
			log.Fatalf("failed to listen on address %s: %v", tlsListenAddr, err)
		}
		defer listener.Close()
		log.Printf("Listening on address %s for TLS endpoint", tlsListenAddr)

		go func() {
			server := http.Server{
				Handler:   muxedHandler,
				TLSConfig: serverSideTLSCfg,
			}
			log.Printf("Serving HTTPS requests on address %s", listener.Addr())
			if err := server.Serve(listener); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.Fatalf("failed to serve: %v", err)
				}
				log.Println("Server exitted")
			}
			go func() {
				<-ctx.Done()
				log.Println("Shutting down server")
				server.Shutdown(ctx)
			}()
		}()
	}

	if plainHTTPListenAddr := agentCmd.HTTPListenAddress; plainHTTPListenAddr != "" {
		listener, err := net.Listen("tcp", plainHTTPListenAddr)
		if err != nil {
			log.Fatalf("failed to listen on address %s: %v", plainHTTPListenAddr, err)
		}
		defer listener.Close()
		log.Printf("Listening on address %s for plaintext HTTP requests", plainHTTPListenAddr)
		go func() {
			log.Printf("Serving plaintext HTTP requests on address %s", listener.Addr())
			server := &http.Server{
				Handler: muxedHandler,
			}
			if err := server.Serve(listener); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.Fatalf("failed to serve: %v", err)
				}
				log.Println("Server exitted")
			}
			go func() {
				<-ctx.Done()
				log.Println("Shutting down server")
				server.Shutdown(ctx)
			}()
		}()
	}

	if nodeName := agentCmd.NodeName; nodeName != "" {
		if agentCmd.ServerAddress != "" || agentCmd.QUICServerAddress != "" {
			log.Printf("Running in cluster mode, acting as an agent, will advertise self as: %s", nodeName)
			if wsServer := agentCmd.ServerAddress; wsServer != "" {
				log.Printf("Hub's WebSocket server: %s", wsServer)
			}
			if quicServer := agentCmd.QUICServerAddress; quicServer != "" {
				log.Printf("Hub's QUIC server: %s", quicServer)
			}
			attributes := make(pkgconnreg.ConnectionAttributes)
			attributes[pkgnodereg.AttributeKeyPingCapability] = "true"
			attributes[pkgnodereg.AttributeKeyNodeName] = nodeName

			if httpEndpoint := agentCmd.HttpEndpoint; httpEndpoint != "" {
				log.Printf("Advertising HTTP endpoint: %s", httpEndpoint)
				attributes[pkgnodereg.AttributeKeyHttpEndpoint] = httpEndpoint
			}

			if exactLoc := agentCmd.ExactLocationLatLon; exactLoc != "" {
				log.Printf("Advertising exact location: %s", exactLoc)
				attributes[pkgnodereg.AttributeKeyExactLocation] = exactLoc
			}

			if alpha2 := agentCmd.CountryCode; alpha2 != "" {
				log.Printf("Advertising country code: %s", alpha2)
				attributes[pkgnodereg.AttributeKeyCountryCode] = alpha2
			}

			if city := agentCmd.CityName; city != "" {
				log.Printf("Advertising city name: %s", city)
				attributes[pkgnodereg.AttributeKeyCityName] = city
			}

			if asn := agentCmd.ASN; asn != "" {
				log.Printf("Advertising ASN: %s", asn)
				attributes[pkgnodereg.AttributeKeyASN] = asn
			}

			if isp := agentCmd.ISP; isp != "" {
				log.Printf("Advertising ISP: %s", isp)
				attributes[pkgnodereg.AttributeKeyISP] = isp
			}

			if dn42asn := agentCmd.DN42ASN; dn42asn != "" {
				log.Printf("Advertising DN42 ASN: %s", dn42asn)
				attributes[pkgnodereg.AttributeKeyDN42ASN] = dn42asn
			}

			if dn42isp := agentCmd.DN42ISP; dn42isp != "" {
				log.Printf("Advertising DN42 ISP: %s", dn42isp)
				attributes[pkgnodereg.AttributeKeyDN42ISP] = dn42isp
			}

			if len(agentCmd.RespondRange) > 0 {
				respondRange := strings.Join(agentCmd.RespondRange, ",")
				log.Printf("Advertising IP respond range: %s", respondRange)
				attributes[pkgnodereg.AttributeKeyRespondRange] = respondRange
			}

			if len(agentCmd.DomainRespondRange) > 0 {
				// the domain respond range involved complex regex string literals, so better encode it somehow before transmitting it over the wire.
				rangesJsonB, err := json.Marshal(agentCmd.DomainRespondRange)
				if err != nil {
					log.Fatalf("failed to marshal domain respond range: %v", err)
				}
				rangeJSON := string(rangesJsonB)
				log.Printf("Advertising domain respond range: %s", rangeJSON)
				attributes[pkgnodereg.AttributeKeyDomainRespondRange] = rangeJSON
			}

			if agentCmd.SupportUDP {
				attributes[pkgnodereg.AttributeKeySupportUDP] = "true"
			}

			if agentCmd.SupportPMTU {
				attributes[pkgnodereg.AttributeKeySupportPMTU] = "true"
			}

			if agentCmd.SupportTCP {
				attributes[pkgnodereg.AttributeKeySupportTCP] = "true"
			}

			if agentCmd.SupportDNS {
				attributes[pkgnodereg.AttributeKeyDNSProbeCapability] = "true"
			}

			if agentCmd.SupportHTTP {
				attributes[pkgnodereg.AttributeKeyHTTPProbeCapability] = "true"
			}

			if quicAddr := agentCmd.QUICServerAddress; quicAddr != "" {
				attributes[pkgnodereg.AttributeKeySupportQUICTunnel] = "true"
			}

			versionJ, _ := json.Marshal(sharedCtx.BuildVersion)
			attributes[pkgnodereg.AttributeKeyVersion] = string(versionJ)

			agent := pkgnodereg.NodeRegistrationAgent{
				HTTPMuxer:         muxedHandler,
				ServerAddress:     agentCmd.ServerAddress,
				QUICServerAddress: agentCmd.QUICServerAddress,
				UseQUIC:           agentCmd.QUICServerAddress != "",
				NodeName:          agentCmd.NodeName,
				ClientCert:        agentCmd.ClientCert,
				ClientCertKey:     agentCmd.ClientCertKey,
				LogEchoReplies:    agentCmd.LogEchoReplies,
				ServerName:        agentCmd.ServerName,
				CustomCertPool:    customCAs,
			}

			if token := agentCmd.getJWTToken(); token != "" {
				agent.Token = &token
			}

			agent.TickInterval, err = time.ParseDuration(agentCmd.AgentTickInterval)
			if err != nil {
				log.Fatalf("failed to parse agent tick interval: %v", err)
			}

			if customTickIntv := agentCmd.AgentTickInterval; customTickIntv != "" {
				intv, err := time.ParseDuration(customTickIntv)
				if err == nil && int64(intv) >= int64(minTickInterval) {
					agent.TickInterval = intv
				}
			}
			log.Printf("Agent tick interval: %s", agent.TickInterval.String())

			agent.NodeAttributes = attributes
			log.Println("Node attributes will be announced as:", attributes)

			log.Println("Initializing node registration agent...")
			if err = agent.Init(); err != nil {
				log.Fatalf("Failed to initialize agent: %v", err)
			}

			log.Println("Starting node registration agent...")

			go func() {
				for {
					nodeRegAgentErrCh := agent.Run(ctx)
					if err, ok := <-nodeRegAgentErrCh; ok && err != nil {
						log.Printf("Node registration agent exited with error: %v, restarting...", err)
						time.Sleep(3 * time.Second)
						continue
					}
					log.Println("Node registration agent exited normally")
					return
				}

			}()
		}
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	log.Printf("Received signal: %v, exiting...", sig.String())
	cancel()

	return nil
}
