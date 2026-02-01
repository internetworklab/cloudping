package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"crypto/x509"
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
	quicConn, err := quicGo.DialAddr(ctx, "127.0.0.1:18447", tlsConfig, nil)
	if err != nil {
		log.Fatalf("failed to dial QUIC address: %v", err)
	}
	log.Printf("Dialed QUIC address: %s,", quicConn.RemoteAddr())

	transport := &quicHttp3.Transport{
		Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quicGo.Config) (*quicGo.Conn, error) {
			return quicConn, nil
		},
	}

	go func(quicConn *quicGo.Conn) {
		time.Sleep(3 * time.Second)

		clientConn := transport.NewClientConn(quicConn)
		log.Printf("Obtained raw client connection: %p, remote: %s, local: %s", clientConn, quicConn.RemoteAddr(), quicConn.LocalAddr())

		requestStream, err := clientConn.OpenRequestStream(ctx)
		if err != nil {
			log.Fatalf("failed to open request stream: %v", err)
		}

		streamId := requestStream.StreamID()
		log.Printf("Opened request stream #%d", streamId)

		go func(rawClientConn *quicHttp3.ClientConn) {
			muxer := http.NewServeMux()
			muxer.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Received test hub to agent request")
				fmt.Fprintf(w, "Hello, World! From agent to hub")
			})
			server := &quicHttp3.Server{
				Handler: muxer,
			}

			err = server.ServeQUICConn(quicConn)
			if err != nil {
				log.Fatalf("failed to serve QUIC connection: %v", err)
			}
		}(clientConn)

		defer requestStream.Close()

		testHeaders := http.Header{}
		testHeaders.Set("X-Test1", "value1")
		testHeaders.Set("Authorization", "Bearer token123")

		req, err := http.NewRequest("POST", "http://localhost/testpath/foo", nil)
		if err != nil {
			log.Fatalf("failed to create new request: %v", err)
		}
		req.Header = testHeaders

		log.Printf("Sending http headers to remote stream #%d", streamId)
		err = requestStream.SendRequestHeader(req)
		if err != nil {
			log.Fatalf("failed to send request header: %v", err)
		}

		log.Printf("Sending http body to remote stream #%d", streamId)

		writer := bufio.NewWriter(requestStream)
		for i := 0; i < 100; i++ {
			time.Sleep(5 * time.Second)
			n, err := writer.WriteString("Hello, World! From agent to hub\n")
			if err != nil {
				log.Fatalf("failed to write to heartbeat stream: %v", err)
			}
			log.Printf("Wrote %d bytes to heartbeat stream", n)
			err = writer.Flush()
			if err != nil {
				log.Fatalf("failed to flush heartbeat stream: %v", err)
			}
		}
	}(quicConn)

	muxer := http.NewServeMux()
	muxer.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received test hub to agent request")
		fmt.Fprintf(w, "Hello, World! From agent to hub")
	})
	server := &quicHttp3.Server{
		Handler: muxer,
	}

	err = server.ServeQUICConn(quicConn)
	if err != nil {
		log.Fatalf("failed to serve QUIC connection: %v", err)
	}

}
