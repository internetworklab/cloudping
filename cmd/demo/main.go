package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
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
	TransportEventNameMethod               = "method"
	TransportEventNameURL                  = "url"
	TransportEventNameProto                = "proto"
	TransportEventNameRequestLine          = "request-line"
	TransportEventNameStatus               = "status"
	TransportEventNameSize                 = "size"
	TransportEventNameRequestHeadersStart  = "request-headers-start"
	TransportEventNameRequestHeadersEnd    = "request-headers-end"
	TransportEventNameResponseHeadersStart = "response-headers-start"
	TransportEventNameResponseHeadersEnd   = "response-headers-end"
	TransportEventNameBodyStart            = "body-start"
	TransportEventNameBodyEnd              = "body-end"
	TransportEventNameBodyChunkBase64      = "body-chunk-base64"
)

type TransportEvent struct {
	Type  TransportEventType
	Name  TransportEventName
	Value string
}

func (e *TransportEvent) String() string {
	j, _ := json.Marshal(e)
	return string(j)
}

// loggingTransport wraps an existing http.RoundTripper
type loggingTransport struct {
	underlyingTransport *http.Transport
	eventChan           chan<- TransportEvent
	errChan             <-chan error
}

// RoundTrip implements the http.RoundTripper interface
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 1. Log Request Line: Method, URL, and Protocol
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeRequest,
		Name:  TransportEventNameMethod,
		Value: req.Method,
	}
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeRequest,
		Name:  TransportEventNameURL,
		Value: req.URL.String(),
	}
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeRequest,
		Name:  TransportEventNameProto,
		Value: req.Proto,
	}
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeRequest,
		Name:  TransportEventNameRequestLine,
		Value: fmt.Sprintf("%s %s %s", req.Method, req.URL.RequestURI(), req.Proto),
	}

	// 2. Log Request Headers
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeMetadata,
		Name:  TransportEventNameRequestHeadersStart,
		Value: "---- Start Request Headers ----",
	}
	for name, values := range req.Header {
		t.eventChan <- TransportEvent{
			Type:  TransportEventTypeRequestHeader,
			Name:  TransportEventName(name),
			Value: strings.Join(values, " "),
		}
	}
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeMetadata,
		Name:  TransportEventNameRequestHeadersEnd,
		Value: "---- End Request Headers ----",
	}

	// Execute the actual request using the underlying transport
	resp, err := t.underlyingTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeResponse,
		Name:  TransportEventNameProto,
		Value: resp.Proto,
	}
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeResponse,
		Name:  TransportEventNameStatus,
		Value: fmt.Sprintf("%d %s", resp.StatusCode, resp.Status),
	}
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeMetadata,
		Name:  TransportEventNameResponseHeadersStart,
		Value: "---- Start Response Headers ----",
	}
	for name, values := range resp.Header {
		t.eventChan <- TransportEvent{
			Type:  TransportEventTypeResponseHeader,
			Name:  TransportEventName(name),
			Value: strings.Join(values, " "),
		}
	}
	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeMetadata,
		Name:  TransportEventNameResponseHeadersEnd,
		Value: "---- End Response Headers ----",
	}

	// 3. Log Response Size
	// Note: We must read the body to get the size. To ensure the caller
	// can still read the body, we must "reset" it after reading.
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close() // Close the original body

	t.eventChan <- TransportEvent{
		Type:  TransportEventTypeResponse,
		Name:  TransportEventNameSize,
		Value: fmt.Sprintf("%d", len(bodyBytes)),
	}

	// Restore the body for the caller to read
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return resp, nil
}

const defaultBufSize = 1024

type HTTPProto string

const (
	HTTPProtoHTTP1 HTTPProto = "http/1.1"
	HTTPProtoHTTP2 HTTPProto = "http/2"
	HTTPProtoHTTP3 HTTPProto = "http/3"
)

func sendRequest(ctx context.Context, url string, extraHeaders http.Header, httpProto HTTPProto) (<-chan TransportEvent, <-chan error) {

	eventChan := make(chan TransportEvent)
	errChan := make(chan error)
	go func(ctx context.Context) {
		defer close(eventChan)
		defer close(errChan)

		// Clone the system's default transport
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			errChan <- fmt.Errorf("Could not cast http.DefaultTransport to *http.Transport")
			return
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
			panic("HTTP/3 is not supported")
		default:
			panic("Invalid HTTP protocol")
		}

		customTransport := &loggingTransport{
			underlyingTransport: defaultTransport,
			eventChan:           eventChan,
			errChan:             errChan,
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

		bufSize := defaultBufSize
		buf := make([]byte, bufSize)
		eventChan <- TransportEvent{
			Type:  TransportEventTypeMetadata,
			Name:  TransportEventNameBodyStart,
			Value: "---- Start Response Body ----",
		}
		for {
			n, err := resp.Body.Read(buf)
			if err != nil {
				break
			}

			eventChan <- TransportEvent{
				Type:  TransportEventTypeResponse,
				Name:  TransportEventNameBodyChunkBase64,
				Value: base64.StdEncoding.EncodeToString(buf[:n]),
			}
		}
		eventChan <- TransportEvent{
			Type:  TransportEventTypeResponse,
			Name:  TransportEventNameBodyEnd,
			Value: "---- End Response Body ----",
		}

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

	eventChan, errChan := sendRequest(ctx, url, headers, HTTPProtoHTTP2)

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
