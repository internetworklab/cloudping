package httpprobe

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkgutils "example.com/rbmq-demo/pkg/utils"
	quicGo "github.com/quic-go/quic-go"
	quicHTTP3 "github.com/quic-go/quic-go/http3"
)

type TransportEventType string

const (
	TransportEventTypeConnection     = "connection"
	TransportEventTypeDNSLookup      = "dns-lookup"
	TransportEventTypeRequest        = "request"
	TransportEventTypeRequestHeader  = "request-header"
	TransportEventTypeResponse       = "response"
	TransportEventTypeResponseHeader = "response-header"
	TransportEventTypeMetadata       = "metadata"
)

type TransportEventName string

const (
	TransportEventNameMethod                        = "method"
	TransportEventNameURL                           = "url"
	TransportEventNameProto                         = "proto"
	TransportEventNameDialStarted                   = "dial-started"
	TransportEventNameDialCompleted                 = "dial-completed"
	TransportEventNameDNSLookupStarted              = "dns-lookup-started"
	TransportEventNameDNSLookupCompleted            = "dns-lookup-completed"
	TransportEventNameDNSLookupError                = "dns-lookup-error"
	TransportEventNameDialError                     = "dial-error"
	TransportEventNameRequestLine                   = "request-line"
	TransportEventNameStatus                        = "status"
	TransportEventNameTransferEncoding              = "transfer-encoding"
	TransportEventNameContentLength                 = "content-length"
	TransportEventNameContentType                   = "content-type"
	TransportEventNameRequestHeadersStart           = "request-headers-start"
	TransportEventNameRequestHeadersEnd             = "request-headers-end"
	TransportEventNameResponseHeadersStart          = "response-headers-start"
	TransportEventNameResponseHeadersEnd            = "response-headers-end"
	TransportEventNameSkipMalformedResponseHeader   = "skip-malformed-response-header"
	TransportEventNameResponseHeaderFieldsTruncated = "response-header-fields-truncated"
	TransportEventNameBodyStart                     = "body-start"
	TransportEventNameBodyEnd                       = "body-end"
	TransportEventNameBodyBytesRead                 = "body-bytes-read"
	TransportEventNameBodyChunkBase64               = "body-chunk-base64"
	TransportEventNameBodyReadTruncated             = "body-read-truncated"
)

type TransportEvent struct {
	Type  TransportEventType
	Name  TransportEventName
	Value string
	Date  time.Time
}

type Event struct {
	Transport     *TransportEvent `json:"transport,omitempty"`
	Error         string          `json:"error,omitempty"`
	CorrelationID string          `json:"correlationId"`
}

func (e *TransportEvent) String() string {
	j, _ := json.Marshal(e)
	return string(j)
}

// loggingTransport wraps an existing http.RoundTripper
type loggingTransport struct {
	underlyingTransport http.RoundTripper
	errChan             <-chan error
	headerFieldsLimit   *int
	logger              *Logger
}

const maxHeaderFieldSize = 4 * 1024

type Logger struct {
	evChan chan<- TransportEvent
}

func NewLogger(evChan chan<- TransportEvent) *Logger {
	return &Logger{
		evChan: evChan,
	}
}

func (lg *Logger) Close() {
	close(lg.evChan)
}

func (lg *Logger) Log(Type TransportEventType, Name TransportEventName, Value string) {
	lg.evChan <- TransportEvent{
		Type:  Type,
		Name:  Name,
		Value: Value,
		Date:  time.Now(),
	}
}

// RoundTrip implements the http.RoundTripper interface
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 1. Log Request Line: Method, URL, and Protocol
	t.logger.Log(TransportEventTypeRequest, TransportEventNameMethod, req.Method)
	t.logger.Log(TransportEventTypeRequest, TransportEventNameURL, req.URL.String())
	t.logger.Log(TransportEventTypeRequest, TransportEventNameProto, req.Proto)
	t.logger.Log(TransportEventTypeRequest, TransportEventNameRequestLine, fmt.Sprintf("%s %s %s", req.Method, req.URL.RequestURI(), req.Proto))

	// 2. Log Request Headers
	if req.Header != nil && len(req.Header) > 0 {
		t.logger.Log(TransportEventTypeMetadata, TransportEventNameRequestHeadersStart, "---- Start Request Headers ----")
		for name, values := range req.Header {
			t.logger.Log(TransportEventTypeRequestHeader, TransportEventName(name), strings.Join(values, " "))
		}
		t.logger.Log(TransportEventTypeMetadata, TransportEventNameRequestHeadersEnd, "---- End Request Headers ----")
	}

	// Execute the actual request using the underlying transport
	resp, err := t.underlyingTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	t.logger.Log(TransportEventTypeResponse, TransportEventNameProto, resp.Proto)
	t.logger.Log(TransportEventTypeResponse, TransportEventNameStatus, fmt.Sprintf("%d %s", resp.StatusCode, resp.Status))
	t.logger.Log(TransportEventTypeMetadata, TransportEventNameResponseHeadersStart, "---- Start Response Headers ----")
	numHeaderFieldsRead := 0
	if t.headerFieldsLimit == nil || (t.headerFieldsLimit != nil && *t.headerFieldsLimit > 0) {
		for name, values := range resp.Header {
			val := strings.Join(values, " ")
			if len(name)+len(val) > maxHeaderFieldSize {
				t.logger.Log(TransportEventTypeMetadata, TransportEventNameSkipMalformedResponseHeader, fmt.Sprintf("maxHeaderFieldSize=%d", maxHeaderFieldSize))
				continue
			}
			t.logger.Log(TransportEventTypeResponseHeader, TransportEventName(name), strings.Join(values, " "))
			numHeaderFieldsRead++
			if t.headerFieldsLimit != nil && numHeaderFieldsRead >= *t.headerFieldsLimit {
				t.logger.Log(TransportEventTypeMetadata, TransportEventNameResponseHeaderFieldsTruncated, fmt.Sprintf("read=%d,limit=%d", numHeaderFieldsRead, *t.headerFieldsLimit))
				break
			}
		}
	}
	t.logger.Log(TransportEventTypeMetadata, TransportEventNameResponseHeadersEnd, "---- End Response Headers ----")

	t.logger.Log(TransportEventTypeResponse, TransportEventNameTransferEncoding, strings.Join(resp.TransferEncoding, " "))
	t.logger.Log(TransportEventTypeResponse, TransportEventNameContentLength, fmt.Sprintf("%d", resp.ContentLength))
	t.logger.Log(TransportEventTypeResponse, TransportEventNameContentType, resp.Header.Get("Content-Type"))

	return resp, nil
}

const defaultBufSize = 1024

type HTTPProto string

const (
	HTTPProtoHTTP1 HTTPProto = "http/1.1"
	HTTPProtoHTTP2 HTTPProto = "http/2"
	HTTPProtoHTTP3 HTTPProto = "http/3"
)

func GetAcceptableHTTPProtos() []HTTPProto {
	return []HTTPProto{HTTPProtoHTTP1, HTTPProtoHTTP2, HTTPProtoHTTP3}
}

func defaultTransportDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return dialer.DialContext
}

func doDNSLookup(ctx context.Context, logger *Logger, resolver *net.Resolver, network string, addr string, pref *InetFamilyPreference) (string, error) {
	prefUsed := "ip"
	if pref != nil {
		prefUsed = string(*pref)
	}
	logger.Log(TransportEventTypeDNSLookup, TransportEventNameDNSLookupStarted, fmt.Sprintf("network=%s,addr=%s,pref=%s", network, addr, prefUsed))

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		err := fmt.Errorf("failed to split host and port from addr: %s: %v", addr, err)
		logger.Log(TransportEventTypeDNSLookup, TransportEventNameDNSLookupError, err.Error())
		return "", err
	}

	ips, err := resolver.LookupIP(ctx, prefUsed, host)
	if err != nil {
		err := fmt.Errorf("failed to lookup ip from host %s: %v", host, err)
		logger.Log(TransportEventTypeDNSLookup, TransportEventNameDNSLookupError, err.Error())
		return "", err
	}

	if len(ips) == 0 {
		err := fmt.Errorf("no ip found for host %s", host)
		logger.Log(TransportEventTypeDNSLookup, TransportEventNameDNSLookupError, err.Error())
		return "", err
	}

	ipStrings := make([]string, 0)
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}
	usedIP := ips[0]
	reJoinedAddr := net.JoinHostPort(usedIP.String(), port)
	logger.Log(TransportEventTypeDNSLookup, TransportEventNameDNSLookupCompleted, fmt.Sprintf("network=%s,addr=%s,ips=%v,usedIP=%s,usedAddr=%s", network, addr, strings.Join(ipStrings, " "), usedIP.String(), reJoinedAddr))

	return reJoinedAddr, nil
}

func getHTTP3Transport(logger *Logger, resolver *net.Resolver, pref *InetFamilyPreference) (*quicHTTP3.Transport, error) {
	tr := &quicHTTP3.Transport{}

	dialFunc := func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quicGo.Config) (*quicGo.Conn, error) {
		nw := "quic"

		resolvedAddr, err := doDNSLookup(ctx, logger, resolver, nw, addr, pref)
		if err != nil {
			return nil, err
		}

		logger.Log(TransportEventTypeConnection, TransportEventNameDialStarted, fmt.Sprintf("network=%s,addr=%s,resolvedAddr=%s", nw, addr, resolvedAddr))

		conn, err := quicGo.DialAddr(ctx, resolvedAddr, tlsCfg, cfg)
		if err != nil {
			logger.Log(TransportEventTypeConnection, TransportEventNameDialError, err.Error())
		} else {
			logger.Log(TransportEventTypeConnection, TransportEventNameDialCompleted, fmt.Sprintf("network=%s,addr=%s,remoteAddr=%+v,localAddr=%+v", nw, addr, conn.RemoteAddr(), conn.LocalAddr()))
		}
		return conn, err
	}
	tr.Dial = dialFunc

	return tr, nil
}

func getTransport(httpProto HTTPProto, logger *Logger, resolver *net.Resolver, pref *InetFamilyPreference) (http.RoundTripper, error) {
	// Clone the system's default transport
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("Could not cast http.DefaultTransport to *http.Transport")
	}

	if defaultTransport.DialContext == nil {
		defaultTransport.DialContext = defaultTransportDialContext(&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		})
	}
	originDialContext := defaultTransport.DialContext

	defaultTransport = defaultTransport.Clone()
	if defaultTransport.Protocols == nil {
		defaultTransport.Protocols = &http.Protocols{}
	}

	defaultTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {

		resolvedAddr, err := doDNSLookup(ctx, logger, resolver, network, addr, pref)
		if err != nil {
			return nil, err
		}

		logger.Log(TransportEventTypeConnection, TransportEventNameDialStarted, fmt.Sprintf("network=%s,addr=%s,resolvedAddr=%s", network, addr, resolvedAddr))

		conn, err := originDialContext(ctx, network, resolvedAddr)
		if err != nil {
			logger.Log(TransportEventTypeConnection, TransportEventNameDialError, err.Error())
		} else {
			logger.Log(TransportEventTypeConnection, TransportEventNameDialCompleted, fmt.Sprintf("network=%s,addr=%s,remoteAddr=%+v,localAddr=%+v", network, addr, conn.RemoteAddr(), conn.LocalAddr()))
		}
		return conn, err
	}

	defaultTransport.TLSClientConfig = &tls.Config{
		NextProtos: []string{"http/1.1"},
	}

	defaultTransport.Protocols.SetHTTP1(false)
	defaultTransport.Protocols.SetHTTP2(false)
	defaultTransport.Protocols.SetUnencryptedHTTP2(false)
	defaultTransport.ForceAttemptHTTP2 = false

	switch httpProto {
	case HTTPProtoHTTP1:
		defaultTransport.Protocols.SetHTTP1(true)
		defaultTransport.ForceAttemptHTTP2 = false
	case HTTPProtoHTTP2:
		defaultTransport.Protocols.SetHTTP2(true)
		defaultTransport.ForceAttemptHTTP2 = true
		defaultTransport.TLSClientConfig.NextProtos = []string{"h2"}
	case HTTPProtoHTTP3:
		return getHTTP3Transport(logger, resolver, pref)
	default:
		panic("Invalid HTTP protocol")
	}
	return defaultTransport, nil
}

type InetFamilyPreference string

const (
	InetFamilyPreferenceIPv4 InetFamilyPreference = "ip4"
	InetFamilyPreferenceIPv6 InetFamilyPreference = "ip6"
	InetFamilyPreferenceDual InetFamilyPreference = "ip"
)

type HTTPProbe struct {
	// Something like 'https://www.google.com/robots.txt' or just 'http://example.com'
	URL string `json:"url"`

	// A dictionary of string slices, e.g. { "X-Forwarded-For": ["127.0.0.1"], "X-Real-IP": ["127.0.0.1"] }
	ExtraHeaders http.Header `json:"extraHeaders,omitempty"`

	// Acceptable values: 'http/1.1', 'http/2', 'http/3', default is 'http/1.1'.
	Proto *HTTPProto `json:"proto,omitempty"`

	// Limit of body bytes to read, default is nil (no limit).
	SizeLimit *int64 `json:"sizeLimit,omitempty"`

	// A custom resolver to use, if it's nil, system's default resolver will be used,
	// format like '1.1.1.1:53' or '8.8.8.8:53', if it's of ipv6, bracket should be surrounded, e.g. '[2001:4860:4860::8888]:53'
	Resolver *string `json:"resolver,omitempty"`

	// Acceptable values: 'ip4', 'ip6', 'ip', default is 'ip'.
	IPPref *InetFamilyPreference `json:"inetFamilyPreference,omitempty"`

	// limit the number of response headers fields, too many header fields will be ignored.
	NumHeadersFieldsLimit *int `json:"numHeadersFieldsLimit,omitempty"`

	// A correlation id is use to correlate the events with the request that generated them.
	// When multiple requests are doing concurrently, one can use this field to determine which request the event belongs to.
	CorrelationID string `json:"correlationId,omitempty"`
}

func (probe *HTTPProbe) Do(ctx context.Context) <-chan Event {

	outEVChan := make(chan Event)

	go func(ctx context.Context) {
		eventChan, errChan := sendRequest(ctx, *probe)
		defer close(outEVChan)

		for {
			select {
			case ev, ok := <-eventChan:
				if !ok {
					return
				}
				outEVChan <- Event{
					Transport:     &ev,
					CorrelationID: probe.CorrelationID,
				}
			case err, ok := <-errChan:
				if !ok {
					return
				}
				outEVChan <- Event{
					Error:         err.Error(),
					CorrelationID: probe.CorrelationID,
				}
			}
		}
	}(ctx)

	return outEVChan
}

func sendRequest(ctx context.Context, probe HTTPProbe) (<-chan TransportEvent, <-chan error) {
	url := probe.URL
	extraHeaders := probe.ExtraHeaders
	var httpProto HTTPProto = HTTPProtoHTTP1
	if probe.Proto != nil {
		httpProto = *probe.Proto
	}

	eventChan := make(chan TransportEvent)
	errChan := make(chan error)

	go func(ctx context.Context) {

		defer close(errChan)

		logger := NewLogger(eventChan)
		defer logger.Close()

		defaultTransport, err := getTransport(httpProto, logger, pkgutils.NewCustomResolver(probe.Resolver, 10*time.Second), probe.IPPref)
		if err != nil {
			errChan <- err
			return
		}

		customTransport := &loggingTransport{
			underlyingTransport: defaultTransport,
			errChan:             errChan,
			headerFieldsLimit:   probe.NumHeadersFieldsLimit,
			logger:              logger,
		}

		// Create a client using the custom transport
		client := &http.Client{
			Transport: customTransport,
			Timeout:   10 * time.Second,
		}

		// Test request
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		if extraHeaders != nil {
			req.Header = extraHeaders
		}

		resp, err := client.Do(req)
		if err != nil {
			errChan <- err
			return
		}

		logger.Log(TransportEventTypeMetadata, TransportEventNameBodyStart, "---- Start Response Body ----")
		var sizeLimit int64 = 0
		if probe.SizeLimit != nil {
			sizeLimit = *probe.SizeLimit
		}
		var bodyBytesRead int64 = 0
		for {
			var bufSize int64 = defaultBufSize
			if probe.SizeLimit != nil {
				remainCapacity := sizeLimit - bodyBytesRead
				if remainCapacity >= 0 && bufSize >= remainCapacity {
					bufSize = remainCapacity
				}
			}
			if bufSize <= 0 {
				break
			}
			buf := make([]byte, bufSize)
			n, err := resp.Body.Read(buf)
			if err != nil {
				break
			}

			logger.Log(TransportEventTypeResponse, TransportEventNameBodyChunkBase64, base64.StdEncoding.EncodeToString(buf[:n]))

			bodyBytesRead += int64(n)
			if probe.SizeLimit != nil && bodyBytesRead >= sizeLimit {
				logger.Log(TransportEventTypeResponse, TransportEventNameBodyReadTruncated, fmt.Sprintf("read=%d,limit=%d", bodyBytesRead, sizeLimit))
				break
			}
		}
		logger.Log(TransportEventTypeResponse, TransportEventNameBodyEnd, "---- End Response Body ----")
		logger.Log(TransportEventTypeResponse, TransportEventNameBodyBytesRead, strconv.FormatInt(bodyBytesRead, 10))

	}(ctx)

	return eventChan, errChan
}
