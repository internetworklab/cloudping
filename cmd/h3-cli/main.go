package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"crypto/x509"
	"io"
	"net/http"
	"os"

	quicGo "github.com/quic-go/quic-go"
	quicHttp3 "github.com/quic-go/quic-go/http3"
)

func main() {
	caPath := "/root/services/globalping/agent/certs/ca.pem"
	caPool := x509.NewCertPool()
	caCertData, err := os.ReadFile(caPath)
	if err != nil {
		log.Fatalf("failed to read CA certificate: %v", err)
	}
	if ok := caPool.AppendCertsFromPEM(caCertData); !ok {
		log.Fatalf("failed to append CA certificate to pool")
	}
	log.Printf("Appended CA certificate %s to ad-hoc cert pool", caPath)

	tlsConfig := &tls.Config{
		RootCAs:            caPool,
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}

	ctx := context.Background()
	quicConn, err := quicGo.DialAddr(ctx, "127.0.0.1:18443", tlsConfig, nil)
	if err != nil {
		log.Fatalf("failed to dial QUIC address: %v", err)
	}
	log.Printf("Dialed QUIC address: %s,", quicConn.RemoteAddr())

	go func(quicConn *quicGo.Conn) {
		muxer := http.NewServeMux()
		muxer.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Received test hub to agent request")
			fmt.Fprintf(w, "Hello, World! From agent to hub")
		})
		server := &quicHttp3.Server{
			Handler: muxer,
		}
		rawServerConn, err := server.NewRawServerConn(quicConn)
		if err != nil {
			log.Fatalf("failed to create raw server connection: %v", err)
		}
		log.Printf("Created raw server connection: %p", rawServerConn)

		log.Printf("Listening hub to agent calls")
		for {
			log.Printf("Accepting hub to agent stream")
			stream, err := quicConn.AcceptStream(ctx)
			if err != nil {
				log.Printf("failed to accept hub to agent stream: %v", err)
				break
			}
			log.Printf("Accepted hub to agent stream: %p %d", stream, stream.StreamID())
			rawServerConn.HandleRequestStream(stream)
		}
	}(quicConn)

	tr := &quicHttp3.Transport{
		TLSClientConfig: tlsConfig,
	}

	rawClientConn := tr.NewRawClientConn(quicConn)
	if rawClientConn == nil {
		log.Fatalf("failed to create QUIC client connection")
	}

	log.Printf("Obtained QUIC Raw client connection: %p", rawClientConn)

	cli := &http.Client{
		Transport: rawClientConn,
	}
	log.Printf("Created HTTP client with QUIC transport: %p", cli)

	httpReq, err := http.NewRequest("GET", "https://127.0.0.1:18443/register", nil)
	if err != nil {
		log.Fatalf("failed to create HTTP request: %v", err)
	}

	resp, err := cli.Do(httpReq)
	if err != nil {
		log.Fatalf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read response body: %v", err)
	}

	log.Printf("Response: %s", string(body))

	time.Sleep(10 * time.Second)
}
