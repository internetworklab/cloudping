package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	quicHTTP3 "github.com/quic-go/quic-go/http3"
)

type TransportEventType string

const (
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
	TransportEventNameRequestLine                   = "request-line"
	TransportEventNameStatus                        = "status"
	TransportEventNameTransferEncoding              = "transfer-encoding"
	TransportEventNameContentLength                 = "content-length"
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
	t.logger.Log(TransportEventTypeMetadata, TransportEventNameRequestHeadersStart, "---- Start Request Headers ----")
	for name, values := range req.Header {
		t.logger.Log(TransportEventTypeRequestHeader, TransportEventName(name), strings.Join(values, " "))
	}
	t.logger.Log(TransportEventTypeMetadata, TransportEventNameRequestHeadersEnd, "---- End Request Headers ----")

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

	return resp, nil
}

const defaultBufSize = 1024

type HTTPProto string

const (
	HTTPProtoHTTP1 HTTPProto = "http/1.1"
	HTTPProtoHTTP2 HTTPProto = "http/2"
	HTTPProtoHTTP3 HTTPProto = "http/3"
)

func getTransport(httpProto HTTPProto) (http.RoundTripper, error) {
	// Clone the system's default transport
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("Could not cast http.DefaultTransport to *http.Transport")
	}

	defaultTransport = defaultTransport.Clone()
	if defaultTransport.Protocols == nil {
		defaultTransport.Protocols = &http.Protocols{}
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
		tr := &quicHTTP3.Transport{}
		return tr, nil
	default:
		panic("Invalid HTTP protocol")
	}
	return defaultTransport, nil
}

type HTTPProbe struct {
	URL          string
	ExtraHeaders http.Header
	Proto        HTTPProto
	SizeLimit    *int64

	// limit the number of response headers fields, too many header fields will be ignored
	NumHeadersFieldsLimit *int
}

func sendRequest(ctx context.Context, probe HTTPProbe) (<-chan TransportEvent, <-chan error) {
	url := probe.URL
	extraHeaders := probe.ExtraHeaders
	httpProto := probe.Proto

	eventChan := make(chan TransportEvent)
	errChan := make(chan error)
	go func(ctx context.Context) {
		defer close(eventChan)
		defer close(errChan)

		defaultTransport, err := getTransport(httpProto)
		if err != nil {
			errChan <- err
			return
		}

		logger := NewLogger(eventChan)
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

func main() {
	headers := http.Header{}
	headers.Set("User-Agent", "Go-Demo-Client/1.0")
	url := "https://www.google.com/robots.txt"

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	maxHeadersFields := 100
	responseSizeLimit := int64(4 * 1024)
	probe := HTTPProbe{
		URL:                   url,
		ExtraHeaders:          headers,
		Proto:                 HTTPProtoHTTP3,
		NumHeadersFieldsLimit: &maxHeadersFields,
		SizeLimit:             &responseSizeLimit,
	}
	eventChan, errChan := sendRequest(ctx, probe)

	log.Println("Starting to listen for events")
	for {
		select {
		case ev, ok := <-eventChan:
			if !ok {
				log.Println("Event channel closed")
				return
			}
			log.Println("Event: ", ev.String())
		case err, ok := <-errChan:
			if !ok {
				log.Println("Error channel closed")
				return
			}
			log.Println("Err: ", err.Error())
		}
	}
}
