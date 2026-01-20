package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"

	"net/http"

	quicGo "github.com/quic-go/quic-go"
	quicHttp3 "github.com/quic-go/quic-go/http3"
)

type MyCtxKey string

const (
	MyCtxKeyConn = MyCtxKey("conn")
)

func main() {
	certPath := "/root/services/globalping/agent/certs/peer.pem"
	certKeyPath := "/root/services/globalping/agent/certs/peer-key.pem"
	certPair, err := tls.LoadX509KeyPair(certPath, certKeyPath)
	if err != nil {
		log.Fatalf("failed to load TLS certificate: %v", err)
	}
	log.Printf("Loaded TLS certificate: %s and key: %s", certPath, certKeyPath)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certPair},
		NextProtos:   []string{"h3"},
	}
	h3Listener, err := quicGo.ListenAddr("0.0.0.0:18443", tlsConfig, nil)
	if err != nil {
		log.Fatalf("failed to listen on address 0.0.0.0:18443: %v", err)
	}
	udpAddr, ok := h3Listener.Addr().(*net.UDPAddr)
	if !ok {
		panic("failed to cast listener address to *net.UDPAddr")
	}
	log.Printf("Listening on UDP address %s", udpAddr.String())

	server := &quicHttp3.Server{
		ConnContext: func(ctx context.Context, c *quicGo.Conn) context.Context {
			ctx = context.WithValue(ctx, MyCtxKeyConn, c)
			log.Printf("Appended Conn into context: %p", c)
			return ctx
		},
	}

	muxer := http.NewServeMux()
	muxer.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		conn := ctx.Value(MyCtxKeyConn).(*quicGo.Conn)
		log.Printf("Conn from context: %p", conn)

		log.Printf("Remote Address From request: %s", r.RemoteAddr)

		if httpStreamer, ok := w.(quicHttp3.HTTPStreamer); ok {
			log.Printf("HTTPStreamer from writer: %p", httpStreamer)
			stream := httpStreamer.HTTPStream()
			log.Printf("HTTPStream from streamer: %p", stream)
			streamId := stream.StreamID()
			log.Printf("Stream ID: %d", streamId)
			defer stream.Close()

			stream.Write([]byte("Hello, World! From stream\n"))
			stream.Write([]byte("Hello, World! From stream\n"))
			stream.Write([]byte("Hello, World! From stream"))

			log.Printf("Calling agent's service /test")
			tr := &quicHttp3.Transport{}
			rawCliConn := tr.NewRawClientConn(conn)
			cli := &http.Client{
				Transport: rawCliConn,
			}
			httpReq, err := http.NewRequest("GET", "https://127.0.0.1:18443/test", nil)
			if err != nil {
				log.Fatalf("failed to create HTTP request: %v", err)
			}
			resp, err := cli.Do(httpReq)
			if err != nil {
				log.Fatalf("failed to call agent's service /test: %v", err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatalf("failed to read response body: %v", err)
			}
			log.Printf("Response from agent's service /test: %s", string(body))

			return
		}

		fmt.Fprintf(w, "Hello, World!")
	})

	server.Handler = muxer

	for {
		conn, err := h3Listener.Accept(context.Background())
		if err != nil {
			log.Fatalf("failed to accept connection: %v", err)
		}

		log.Printf("Accepted connection: %p %s", conn, conn.RemoteAddr())
		rawServerConn, err := server.NewRawServerConn(conn)
		if err != nil {
			log.Fatalf("failed to obtain raw server connection: %v", err)
		}

		go func(conn *quicGo.Conn, rawServerConn *quicHttp3.RawServerConn) {
			defer log.Printf("Closing connection: %p %s", conn, conn.RemoteAddr())
			for {
				log.Printf("Accepting stream from connection: %p %s", conn, conn.RemoteAddr())
				stream, err := conn.AcceptStream(context.Background())
				if err != nil {
					log.Printf("failed to accept stream: %v", err)
					break
				}
				log.Printf("Accepted stream: %p %d from connection: %s", stream, stream.StreamID(), conn.RemoteAddr())

				log.Printf("Handling stream: %p %d from connection: %s", stream, stream.StreamID(), conn.RemoteAddr())
				rawServerConn.HandleRequestStream(stream)

			}
		}(conn, rawServerConn)
	}
}
