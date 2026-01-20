package main

import (
	"context"
	"crypto/tls"
	"fmt"
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
	log.Printf("Created QUIC HTTP3 Server %p", server)

	muxer := http.NewServeMux()
	muxer.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		server := ctx.Value(quicHttp3.ServerContextKey).(*quicHttp3.Server)
		log.Printf("Server from context: %p", server)

		conn := ctx.Value(MyCtxKeyConn).(*quicGo.Conn)
		log.Printf("Conn from context: %p", conn)

		log.Printf("Remote Address From connection: %s", conn.RemoteAddr())
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

			return
		}

		fmt.Fprintf(w, "Hello, World!")
	})

	server.Handler = muxer
	err = server.ServeListener(h3Listener)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
