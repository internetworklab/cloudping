package dnsprobe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	"codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
	quicHTTP3 "github.com/quic-go/quic-go/http3"
)

type Transport string

const (
	TransportUDP   Transport = "udp"
	TransportTCP   Transport = "tcp"
	TransportTLS   Transport = "tls"    // DNS over TLS, defined by RFC7858
	TransportHTTP2 Transport = "http/2" // RFC8484, HTTP/2, POST method, application/dns-message wire format
	TransportHTTP3 Transport = "http/3" // RFC8484, HTTP/3, POST method, application/dns-message wire format
)

type DNSQueryType string

const (
	DNSQueryTypeA     DNSQueryType = "a"
	DNSQueryTypeAAAA  DNSQueryType = "aaaa"
	DNSQueryTypeCNAME DNSQueryType = "cname"
	DNSQueryTypeMX    DNSQueryType = "mx"
	DNSQueryTypeNS    DNSQueryType = "ns"
	DNSQueryTypePTR   DNSQueryType = "ptr"
	DNSQueryTypeTXT   DNSQueryType = "txt"
)

const defaultDNSProbeTransport = TransportUDP

type LookupParameter struct {
	CorrelationID string `json:"corrId,omitempty"`

	// For UDP, or TCP transport, valid addrPort are ip addresses or <ip>:<port>, [<ipv6>]:<port>
	// e.g. 1.1.1.1, 1.1.1.1:53, 2606:4700:4700::1111, [2606:4700:4700::1111]:53
	// For DoT, valid addrPort including valid addrPort for UDP, TCP transport plus a `tls://` prefix,
	// e.g. tls://1.1.1.1, 1.1.1.1, 1.1.1.1:53, 2606:4700:4700::1111, [2606:4700:4700::1111]:53
	// For DoH, valid addrPort should be an HTTPS URL, but the host part must be ip or ipv6 address literal
	// e.g. https://8.8.8.8/dns-query, or https://[2001:4860:4860::8888]/dns-query, ipv6 address literal must be wrapped within a bracket pair.
	AddrPort      string       `json:"addrport"`
	Target        string       `json:"target"`
	TimeoutMs     *int64       `json:"timeoutMs,omitempty"`
	Transport     *Transport   `json:"transport,omitempty"`
	QueryType     DNSQueryType `json:"queryType"`
	DoTServerName string       `json:"dotServerName"`
}

type QueryResult struct {
	CorrelationID    string        `json:"corrId,omitempty"`
	Server           string        `json:"server"`
	Target           string        `json:"target,omitempty"`
	QueryType        DNSQueryType  `json:"query_type,omitempty"`
	Answers          []interface{} `json:"answers,omitempty"`
	AnswerStrings    []string      `json:"answer_strings,omitempty"`
	Error            error         `json:"error,omitempty"`
	ErrString        string        `json:"err_string,omitempty"`
	IOTimeout        bool          `json:"io_timeout,omitempty"`
	NoSuchHost       bool          `json:"no_such_host,omitempty"`
	Elapsed          time.Duration `json:"elapsed,omitempty"`
	StartedAt        time.Time     `json:"started_at"`
	TimeoutSpecified time.Duration `json:"timeout_specified"`
	TransportUsed    Transport     `json:"transport_used"`
}

// make it suitable for transmitting over the wire
func (qr *QueryResult) PreStringify() (*QueryResult, error) {
	clone := new(QueryResult)
	*clone = *qr
	if clone.Error != nil {
		clone.ErrString = clone.Error.Error()
		clone.Error = nil
	}

	if clone.Answers != nil && qr.TransportUsed != TransportHTTP2 && qr.TransportUsed != TransportHTTP3 {
		answerStrings := make([]string, 0)
		for _, ans := range clone.Answers {
			ansStr, err := wrappedAnsToString(ans, clone.QueryType)
			if err != nil {
				return nil, fmt.Errorf("failed to convert answer to string: %v", err)
			}
			answerStrings = append(answerStrings, ansStr)
		}
		clone.AnswerStrings = answerStrings
		clone.Answers = nil
	}
	return clone, nil
}

func analyzeError(err error, queryResult *QueryResult) bool {
	queryResult.Error = err
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		queryResult.IOTimeout = true
		return true
	} else if errors.Is(err, context.Canceled) {
		queryResult.IOTimeout = true
		return true
	} else if _, ok := err.(*net.DNSError); ok {
		queryResult.NoSuchHost = true
		return true
	} else {
		return false
	}
}

func stripTLSURLPrefix(s string) string {
	if after, ok := strings.CutPrefix(s, "tls://"); ok {
		return after
	}
	return s
}

func appendDNSPort(s string, transport Transport) string {
	port := "53"
	if transport == TransportTLS {
		// As per RFC7853, https://datatracker.ietf.org/doc/html/rfc7858#section-3.1
		port = "853"
	}
	_, _, err := net.SplitHostPort(s)
	if err != nil {
		return net.JoinHostPort(s, port)
	}
	return s
}

const minTimeoutMs = 10
const maxTimeoutMs = 10 * 1000
const defaultDNSProbeTimeoutMs = 3000

func getDoHRequest(ctx context.Context, urlStr string, m *dns.Msg, serverName string) (*http.Request, error) {

	mime := "application/dns-message"

	if err := m.Pack(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, m)
	if err != nil {
		return nil, err
	}

	if serverName != "" {
		// Note, such serverName is actually the Host (or :authority) header field
		req.Host = serverName
	}

	req.Header.Set("Accept", mime)
	req.Header.Set("Content-Type", mime)
	return req, nil
}

// returns: answers, error
func LookupDNS(ctx context.Context, parameter LookupParameter, certPool *x509.CertPool) (*QueryResult, error) {

	var transport Transport = defaultDNSProbeTransport
	if parameter.Transport != nil {
		transport = *parameter.Transport
	}

	target := parameter.Target

	var timeoutMs int64 = defaultDNSProbeTimeoutMs
	if parameter.TimeoutMs != nil {
		timeoutMs = *parameter.TimeoutMs
	}

	if timeoutMs < minTimeoutMs {
		return nil, fmt.Errorf("timeout is too short: at least %dms is required, got %dms", minTimeoutMs, parameter.TimeoutMs)
	}
	if timeoutMs > maxTimeoutMs {
		return nil, fmt.Errorf("timeout is too long: at most %dms is allowed, got %dms", maxTimeoutMs, parameter.TimeoutMs)
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond

	queryType := parameter.QueryType
	queryResult := new(QueryResult)
	queryResult.Target = target
	queryResult.QueryType = queryType
	queryResult.Answers = make([]interface{}, 0)
	queryResult.Server = parameter.AddrPort
	queryResult.TimeoutSpecified = timeout
	queryResult.TransportUsed = transport
	queryResult.CorrelationID = parameter.CorrelationID

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: parameter.DoTServerName,
	}
	if certPool != nil {
		tlsConfig.RootCAs = certPool
	}

	queryResult.StartedAt = time.Now()
	defer func() {
		queryResult.Elapsed = time.Since(queryResult.StartedAt)
	}()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if transport == TransportHTTP2 || transport == TransportHTTP3 {
		var m *dns.Msg = nil
		switch queryType {
		case DNSQueryTypeA:
			m = dns.NewMsg(target, dns.TypeA)
		case DNSQueryTypeAAAA:
			m = dns.NewMsg(target, dns.TypeAAAA)
		case DNSQueryTypeCNAME:
			m = dns.NewMsg(target, dns.TypeCNAME)
		case DNSQueryTypeMX:
			m = dns.NewMsg(target, dns.TypeMX)
		case DNSQueryTypeNS:
			m = dns.NewMsg(target, dns.TypeNS)
		case DNSQueryTypePTR:
			ipaddr, err := netip.ParseAddr(target)
			if err != nil {
				return nil, fmt.Errorf("invalid ip: %w", err)
			}
			reverseAddr := dnsutil.ReverseAddr(ipaddr)
			m = dns.NewMsg(reverseAddr, dns.TypePTR)
		case DNSQueryTypeTXT:
			m = dns.NewMsg(target, dns.TypeTXT)
		default:
			return nil, fmt.Errorf("invalid query type: %s", queryType)
		}

		m.ID = dns.ID()
		m.RecursionDesired = true

		req, err := getDoHRequest(ctx, parameter.AddrPort, m, parameter.DoTServerName)
		if err != nil {
			return nil, fmt.Errorf("failed to create DoH request: %w", err)
		}

		ansM := new(dns.Msg)
		if transport == TransportHTTP2 {
			cli := http.DefaultClient
			resp, err := cli.Do(req)
			if err != nil {
				return nil, fmt.Errorf("failed to send DoH request via http client: %w", err)
			}
			if resp.StatusCode >= 400 {
				return nil, fmt.Errorf("DoH request failed with status code: %s", resp.Status)
			}
			defer resp.Body.Close()
			ansM.Data, err = io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read DoH response: %w", err)
			}
		} else {
			tr := &quicHTTP3.Transport{
				TLSClientConfig: tlsConfig,
			}
			var err error
			resp, err := tr.RoundTrip(req)
			if err != nil {
				return nil, fmt.Errorf("failed to send DoH request via quic client: %w", err)
			}
			if resp.StatusCode >= 400 {
				return nil, fmt.Errorf("DoH3 request failed with status code: %s", resp.Status)
			}
			defer resp.Body.Close()
			ansM.Data, err = io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read DoH response: %w", err)
			}
		}

		if err := ansM.Unpack(); err != nil {
			return nil, fmt.Errorf("failed to unpack DoH response: %w", err)
		}

		for _, rr := range ansM.Answer {
			rrData := rr.Data()
			ansDataStr := rrData.String()
			queryResult.Answers = append(queryResult.Answers, rrData)
			queryResult.AnswerStrings = append(queryResult.AnswerStrings, ansDataStr)
		}

		return queryResult, nil
	} else {
		addrPort := parameter.AddrPort
		addrPort = stripTLSURLPrefix(addrPort)
		addrPort = appendDNSPort(addrPort, transport)
		addrportObj, err := netip.ParseAddrPort(addrPort)
		if err != nil {
			return nil, fmt.Errorf("failed to parse addrport %s as netip.AddrPort: %v", parameter.AddrPort, err)
		}
		resolver := net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				if transport == TransportUDP {
					udpaddr := net.UDPAddrFromAddrPort(addrportObj)
					if udpaddr == nil {
						return nil, fmt.Errorf("failed to get udpaddr from %s", addrportObj.String())
					}
					return net.DialUDP("udp", nil, udpaddr)
				} else if transport == TransportTCP {
					tcpaddr := net.TCPAddrFromAddrPort(addrportObj)
					if tcpaddr == nil {
						return nil, fmt.Errorf("failed to get tcpaddr from %s", addrportObj.String())
					}
					return net.DialTCP("tcp", nil, tcpaddr)
				} else if transport == TransportTLS {
					tcpaddr := net.TCPAddrFromAddrPort(addrportObj)
					dialer := &tls.Dialer{
						Config: tlsConfig,
					}
					return dialer.DialContext(ctx, "tcp", tcpaddr.String())
				} else {
					return nil, fmt.Errorf("transport is not specified or invalid transport: %s", transport)
				}
			},
		}

		switch queryType {
		case DNSQueryTypeA, DNSQueryTypeAAAA:
			ipPref := "ip"
			if queryType == DNSQueryTypeA {
				ipPref = "ip4"
			} else if queryType == DNSQueryTypeAAAA {
				ipPref = "ip6"
			}
			answers, err := resolver.LookupIP(ctx, ipPref, target)
			if err != nil && !analyzeError(err, queryResult) {
				return nil, fmt.Errorf("failed to lookup ip (type %s) for %s: %v", queryType, target, err)
			}

			for _, ans := range answers {
				queryResult.Answers = append(queryResult.Answers, ans)
			}
			return queryResult, nil
		case DNSQueryTypeCNAME:
			answer, err := resolver.LookupCNAME(ctx, target)
			if err != nil && !analyzeError(err, queryResult) {
				return nil, fmt.Errorf("failed to lookup ip (type %s) for %s: %v", queryType, target, err)
			}

			if answer != "" {
				queryResult.Answers = append(queryResult.Answers, answer)
			}
			return queryResult, nil
		case DNSQueryTypeMX:
			answers, err := resolver.LookupMX(ctx, target)
			if err != nil && !analyzeError(err, queryResult) {
				return nil, fmt.Errorf("failed to lookup mx for %s: %v", target, err)
			}
			for _, ans := range answers {
				if ans == nil {
					continue
				}
				queryResult.Answers = append(queryResult.Answers, ans)
			}
			return queryResult, nil
		case DNSQueryTypeNS:
			answers, err := resolver.LookupNS(ctx, target)
			if err != nil && !analyzeError(err, queryResult) {
				return nil, fmt.Errorf("failed to lookup ns for %s: %v", target, err)
			}
			for _, ans := range answers {
				if ans == nil {
					continue
				}
				queryResult.Answers = append(queryResult.Answers, ans)
			}
			return queryResult, nil
		case DNSQueryTypePTR:
			answer, err := resolver.LookupAddr(ctx, target)
			if err != nil && !analyzeError(err, queryResult) {
				return nil, fmt.Errorf("failed to lookup ptr for %s: %v", target, err)
			}
			for _, ans := range answer {
				if ans == "" {
					continue
				}
				queryResult.Answers = append(queryResult.Answers, ans)
			}
			return queryResult, nil
		case DNSQueryTypeTXT:
			answer, err := resolver.LookupTXT(ctx, target)
			if err != nil && !analyzeError(err, queryResult) {
				return nil, fmt.Errorf("failed to lookup txt for %s: %v", target, err)
			}
			for _, ans := range answer {
				queryResult.Answers = append(queryResult.Answers, ans)
			}
			return queryResult, nil
		default:
			return nil, fmt.Errorf("invalid query type: %s", queryType)
		}
	}

}

func wrappedAnsToString(ans interface{}, qtype DNSQueryType) (string, error) {
	switch qtype {
	case DNSQueryTypeA:
		ip, ok := ans.(net.IP)
		if !ok {
			return "", fmt.Errorf("answer is not a net.IP: %v", ans)
		}
		return ip.String(), nil
	case DNSQueryTypeAAAA:
		ip, ok := ans.(net.IP)
		if !ok {
			return "", fmt.Errorf("answer is not a net.IP: %v", ans)
		}
		return ip.String(), nil
	case DNSQueryTypeCNAME:
		cname, ok := ans.(string)
		if !ok {
			return "", fmt.Errorf("answer is not a string: %v", ans)
		}
		return cname, nil
	case DNSQueryTypeMX:
		mx, ok := ans.(*net.MX)
		if !ok {
			return "", fmt.Errorf("answer is not a *net.MX: %v", ans)
		}
		return fmt.Sprintf("%s (pref=%d)", mx.Host, mx.Pref), nil
	case DNSQueryTypeNS:
		ns, ok := ans.(*net.NS)
		if !ok {
			return "", fmt.Errorf("answer is not a *net.NS: %v", ans)
		}
		return ns.Host, nil
	case DNSQueryTypePTR:
		ptr, ok := ans.(string)
		if !ok {
			return "", fmt.Errorf("answer is not a string: %v", ans)
		}
		return ptr, nil
	default:
		return "", fmt.Errorf("unknown query type: %s", qtype)
	}
}
