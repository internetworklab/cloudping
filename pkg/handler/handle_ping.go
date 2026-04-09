package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	pkghttpprobe "github.com/internetworklab/cloudping/pkg/httpprobe"
	pkgipinfo "github.com/internetworklab/cloudping/pkg/ipinfo"
	pkgmyprom "github.com/internetworklab/cloudping/pkg/myprom"
	pkgpinger "github.com/internetworklab/cloudping/pkg/pinger"
	pkgratelimit "github.com/internetworklab/cloudping/pkg/ratelimit"
	pkgraw "github.com/internetworklab/cloudping/pkg/raw"
	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
)

// This handler is designed to be running on agents rather than on a hub,
// It's responsible for bringing up the concret pinger that actually do things.

type PingHandler struct {
	IPInfoReg             *pkgipinfo.IPInfoProviderRegistry
	RespondRange          []net.IPNet
	DomainRespondRange    []regexp.Regexp
	HTTPProbeAdditionalCA []string
}

func (ph *PingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	pingRequest, err := pkgpinger.ParseSimplePingRequest(r)
	if err != nil {
		json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: err.Error()})
		return
	}

	pingReqJSB, _ := json.Marshal(pingRequest)
	log.Printf("Started ping request for %s: %s", pkgutils.GetRemoteAddr(r), string(pingReqJSB))
	defer log.Printf("Finished ping request for %s: %s", pkgutils.GetRemoteAddr(r), string(pingReqJSB))

	counterStore := r.Context().Value(pkgutils.CtxKeyPrometheusCounterStore).(*pkgmyprom.CounterStore)
	if counterStore == nil {
		panic("failed to obtain counter store from request context")
	}

	commonLabels := prometheus.Labels{
		pkgmyprom.PromLabelFrom:   strings.Join(pingRequest.From, ","),
		pkgmyprom.PromLabelTarget: strings.Join(pingRequest.Targets, ","),
		pkgmyprom.PromLabelClient: pkgutils.GetRemoteAddr(r),
	}

	startedAt := time.Now()
	defer func() {
		servedDurationMs := time.Since(startedAt).Milliseconds()
		counterStore.NumRequestsServed.With(commonLabels).Add(1.0)
		counterStore.ServedDurationMs.With(commonLabels).Add(float64(servedDurationMs))
	}()

	if len(ph.DomainRespondRange) > 0 {
		// if domain respond range is explicitly specified, check if there are any domain vialation
		for _, dest := range pingRequest.Targets {
			if ipObj := net.ParseIP(dest); ipObj != nil {
				// skip non-domain (i.e. IP address) targets
				continue
			}

			if hit := slices.ContainsFunc(ph.DomainRespondRange, func(domainPattern regexp.Regexp) bool {
				return domainPattern.MatchString(dest)
			}); !hit {
				json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("domain %s does not match any pattern in the domain respond range", dest).Error()})
				return
			}
		}
	}

	var ipinfoAdapter pkgipinfo.GeneralIPInfoAdapter = nil
	if pingRequest.IPInfoProviderName != nil && *pingRequest.IPInfoProviderName != "" {
		ipinfoAdapter, err = ph.IPInfoReg.GetAdapter(*pingRequest.IPInfoProviderName)
		if err != nil {
			json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: err.Error()})
			return
		}
	}

	ctx := r.Context()
	var rateLimiterUsed pkgratelimit.RateLimiter = nil
	if rateLimitAny := ctx.Value(pkgutils.CtxKeySharedRateLimitEnforcer); rateLimitAny != nil {
		rateLimit, ok := rateLimitAny.(pkgratelimit.RateLimiter)
		if ok && rateLimit != nil {
			rateLimiterUsed = rateLimit
		}
	}

	ipPref := "ip"
	if pingRequest.PreferV6 != nil && *pingRequest.PreferV6 {
		ipPref = "ip6"
	} else if pingRequest.PreferV4 != nil && *pingRequest.PreferV4 {
		ipPref = "ip4"
	}
	lookupIP := func(host string, resolverEndpoint *string, inetFamilyPreference *pkghttpprobe.InetFamilyPreference) ([]net.IP, error) {
		ipPrefUsed := ipPref
		if inetFamilyPreference != nil && *inetFamilyPreference != "" {
			ipPrefUsed = string(*inetFamilyPreference)
		}

		resolver := net.DefaultResolver
		if pingRequest.Resolver != nil && *pingRequest.Resolver != "" {
			resolver = pkgutils.NewCustomResolver(pingRequest.Resolver, 10*time.Second)
		}

		if resolverEndpoint != nil && *resolverEndpoint != "" {
			resolver = pkgutils.NewCustomResolver(resolverEndpoint, 10*time.Second)
		}

		return resolver.LookupIP(ctx, ipPrefUsed, host)
	}

	var pinger pkgpinger.Pinger = nil
	if pingRequest.L7PacketType != nil {
		switch *pingRequest.L7PacketType {
		case pkgpinger.L7ProtoDNS:
			dnsServers := make([]string, 0)
			for _, tgt := range pingRequest.DNSTargets {

				dnsServerIP, err := pkgutils.GetHost(tgt.AddrPort)
				if err != nil {
					json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Sprintf("failed to parse dns server ip from addrport: %s: %v", tgt.AddrPort, err)})
					return
				}

				if len(ph.RespondRange) > 0 && !pkgutils.CheckIntersectIP(dnsServerIP, ph.RespondRange) {
					json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("dns server ip %s is not in the respond range", dnsServerIP.String()).Error()})
					return
				}

				dnsServers = append(dnsServers, tgt.AddrPort)
			}

			// when in dns mode, we are mainly sending packets to dns servers, so, set targets to dns servers
			commonLabels[pkgmyprom.PromLabelTarget] = strings.Join(dnsServers, ",")

			pinger = &pkgpinger.DNSPinger{
				Requests:    pingRequest.DNSTargets,
				RateLimiter: rateLimiterUsed,
				AddCAPaths:  ph.HTTPProbeAdditionalCA,
			}
		case pkgpinger.L7ProtoHTTP:
			httpUrls := make([]string, 0)
			httpPinger := &pkgpinger.HTTPPinger{
				Requests:    make([]pkghttpprobe.HTTPProbe, 0),
				RateLimiter: rateLimiterUsed,
				AddCA:       ph.HTTPProbeAdditionalCA,
			}
			for _, tgt := range pingRequest.HTTPTargets {
				urlObj, err := url.Parse(tgt.URL)
				if err != nil {
					json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("failed to parse http url %s: %v", tgt.URL, err).Error()})
					return
				}

				host := urlObj.Hostname()
				if len(ph.DomainRespondRange) > 0 {
					ipObj := net.ParseIP(host)
					if ipObj == nil && !pkgutils.CheckDomainInRange(host, ph.DomainRespondRange) {
						json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("host %s does not match any pattern in the domain respond range", host).Error()})
						return
					}
				}

				resolverEndpoint := pingRequest.Resolver
				if tgt.Resolver != nil && *tgt.Resolver != "" {
					resolverEndpoint = tgt.Resolver
				}

				if tgt.Resolver == nil || *tgt.Resolver == "" {
					tgt.Resolver = resolverEndpoint
				}

				if len(ph.RespondRange) > 0 {
					if tgt.Resolver != nil {
						resolverIP := net.ParseIP(*tgt.Resolver)
						if resolverIP == nil {
							json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("failed to parse resolver ip from string: %s", *tgt.Resolver).Error()})
							return
						}
						if !pkgutils.CheckIntersectIP(resolverIP, ph.RespondRange) {
							json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("resolver ip %s is not in the respond range", resolverIP.String()).Error()})
							return
						}
					}

					if tgt.IPPref == nil || *tgt.IPPref == "" {
						tgt.IPPref = new(pkghttpprobe.InetFamilyPreference)
						*tgt.IPPref = pkghttpprobe.InetFamilyPreference(ipPref)
					}

					ips, err := lookupIP(host, resolverEndpoint, tgt.IPPref)
					if err != nil {
						json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("failed to lookup ip for host %s: %v", host, err).Error()})
						return
					}
					if !pkgutils.CheckIntersect(ips, ph.RespondRange) {
						json.NewEncoder(w).Encode(pkgutils.ErrorResponse{Error: fmt.Errorf("ips %v are not in the respond range", ips).Error()})
						return
					}
				}

				if tgt.ExtraHeaders == nil {
					tgt.ExtraHeaders = make(http.Header)
				}
				if tgt.ExtraHeaders.Get("User-Agent") == "" {
					tgt.ExtraHeaders.Set("User-Agent", "cloudping/1.0")
				}
				httpPinger.Requests = append(httpPinger.Requests, tgt)
				httpUrls = append(httpUrls, tgt.URL)
			}

			commonLabels[pkgmyprom.PromLabelTarget] = strings.Join(httpUrls, ",")
			pinger = httpPinger
		}
	} else if pingRequest.L4PacketType != nil && *pingRequest.L4PacketType == pkgpinger.L4ProtoTCP {
		tcpingPinger := &pkgpinger.TCPSYNPinger{
			PingRequest:   pingRequest,
			IPInfoAdapter: ipinfoAdapter,
			RespondRange:  ph.RespondRange,
			OnSent: func(ctx context.Context, srcIP net.IP, srcPort int, dstIP net.IP, dstPort int, nBytes int) {
				counterStore.NumBytesSent.With(commonLabels).Add(float64(nBytes))
			},
			OnReceived: func(ctx context.Context, srcIP net.IP, srcPort int, dstIP net.IP, dstPort int, nBytes int) {
				counterStore.NumBytesReceived.With(commonLabels).Add(float64(nBytes))
			},
			RateLimiter: rateLimiterUsed,
		}

		pinger = tcpingPinger
	} else {
		icmpOrUDPPinger := &pkgpinger.SimplePinger{
			PingRequest:   pingRequest,
			IPInfoAdapter: ipinfoAdapter,
			RespondRange:  ph.RespondRange,
			OnSent: func(ctx context.Context, request *pkgraw.ICMPSendRequest, reply *pkgraw.ICMPReceiveReply, peer string, nBytes int) error {
				counterStore.NumBytesSent.With(commonLabels).Add(float64(nBytes))
				return nil
			},
			OnReceived: func(ctx context.Context, request *pkgraw.ICMPSendRequest, reply *pkgraw.ICMPReceiveReply, peer string, nBytes int) error {
				counterStore.NumBytesReceived.With(commonLabels).Add(float64(nBytes))
				return nil
			},
			RateLimiter: rateLimiterUsed,
		}
		pinger = icmpOrUDPPinger
	}

	ctx = context.WithValue(ctx, pkgutils.CtxKeyPromCommonLabels, commonLabels)
	for ev := range pinger.Ping(ctx) {
		if ev.Error != nil {
			errStr := ev.Error.Error()
			ev.Err = &errStr
		}
		if err := json.NewEncoder(w).Encode(ev); err != nil {
			log.Printf("failed to serialize and send an event: %v", err)
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
