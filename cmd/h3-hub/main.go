package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	pkgutils "example.com/rbmq-demo/pkg/utils"
	quicGo "github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
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
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			remote := pkgutils.GetRemoteAddr(r)

			log.Printf("Received new http request from remote %s, method: %s, url: %s, headers:\n", remote, r.Method, r.URL.String())
			for k, vals := range r.Header {
				log.Printf("  %s: %s", k, strings.Join(vals, ", "))
			}

			if streamer, ok := w.(http3.HTTPStreamer); ok {
				log.Printf("Converted http.ResponseWriter to http3.HTTPStreamer, hijacking connection, remote: %s ...", remote)
				stream := streamer.HTTPStream()
				streamID := stream.StreamID()
				go func(ctx context.Context, stream *quicHttp3.Stream) {
					defer stream.Close()
					scanner := bufio.NewScanner(stream)
					timeoutIntv, _ := time.ParseDuration("30s")
					log.Printf("Started to handling ping/pong stream #%d from client. remote: %s", streamID, remote)
					for {
						select {
						case <-ctx.Done():
							log.Printf("Context done, closing stream #%d of remote %s.", streamID, remote)
							return
						default:
							if err := stream.SetReadDeadline(time.Now().Add(timeoutIntv)); err != nil {
								log.Printf("failed to set read deadline to stream #%d of remote %s: %v", streamID, remote, err)
								return
							}

							if ok := scanner.Scan(); !ok {
								if err := scanner.Err(); err != nil {
									log.Printf("failed to scan stream #%d of remote %s: %v", streamID, remote, err)
									return
								}
								log.Printf("Stream #%d of remote %s closed, closing stream.", streamID, remote)
								return
							}
							line := scanner.Bytes()
							log.Printf("Received line from stream #%d of remote %s: %s", streamID, remote, line)

						}
					}

				}(r.Context(), stream)
			}
			w.WriteHeader(http.StatusOK)
		}),
		ConnContext: func(ctx context.Context, c *quicGo.Conn) context.Context {
			log.Printf("Connection accepted, append connection %p to context", c)
			return context.WithValue(ctx, MyCtxKeyConn, c)
		},
	}
	if err := server.ServeListener(h3Listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
