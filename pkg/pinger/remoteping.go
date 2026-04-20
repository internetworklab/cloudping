package pinger

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

type SimpleRemotePinger struct {
	NodeName           string
	Endpoint           string
	Request            SimplePingRequest
	ClientTLSConfig    *tls.Config
	ExtraRequestHeader map[string]string
	QUICClient         *http.Client
}

func (sp *SimpleRemotePinger) getDefaultClient() *http.Client {
	client := &http.Client{}
	if sp.ClientTLSConfig != nil {
		client.Transport = &http.Transport{
			TLSClientConfig: sp.ClientTLSConfig,
		}
	}
	return client
}

func (sp *SimpleRemotePinger) Ping(ctx context.Context) <-chan PingEvent {
	// return mockPing(ctx)
	evChan := make(chan PingEvent)
	go func() {
		defer close(evChan)

		urlStr := ""
		client := sp.getDefaultClient()
		if sp.QUICClient != nil {

			// actually it's not localhost, it will be sent via the QUIC-based tunnel,
			// the nodeName might appear slash '/' (or backslash '\') which make it unsuitable to be a valid hostname.
			urlStr = "http://localhost/simpleping"

			urlObj, err := url.Parse(urlStr)
			if err != nil {
				log.Printf("failed to parse endpoint: %v", err)
				evChan <- PingEvent{Error: fmt.Errorf("Invalid backend URL: %w", err)}
				return
			}
			urlObj.RawQuery = sp.Request.ToURLValues().Encode()
			urlStr = urlObj.String()
			client = sp.QUICClient
		} else {
			urlObj, err := url.Parse(sp.Endpoint)
			if err != nil {
				log.Printf("failed to parse endpoint: %v", err)
				evChan <- PingEvent{Error: fmt.Errorf("Invalid backend URL: %w", err)}
				return
			}
			urlObj.RawQuery = sp.Request.ToURLValues().Encode()
			urlStr = urlObj.String()
		}

		req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
		if err != nil {
			log.Printf("failed to create request: %v", err)
			evChan <- PingEvent{Error: fmt.Errorf("Failed to create http request to backend: %w", err)}
			return
		}

		if sp.ExtraRequestHeader != nil {
			for k, v := range sp.ExtraRequestHeader {
				req.Header.Set(k, v)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("failed to send request to %s: %v", urlStr, err)
			evChan <- PingEvent{Error: fmt.Errorf("Failed to invoke http request to backend: %w", err)}
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if err := scanner.Err(); err != nil {
				evChan <- PingEvent{Error: fmt.Errorf("Error while line-scaning upstream data: %w", err)}
				return
			}

			line := scanner.Bytes()

			pingEVObj := new(PingEvent)
			if err := json.Unmarshal(line, pingEVObj); err != nil {
				evChan <- PingEvent{Error: fmt.Errorf("Unable to unamrshal upstream JSON response: %w, content (start at nextline):\n%s\n", err, string(line))}
				return
			}
			evChan <- *pingEVObj
		}

	}()

	return evChan
}
