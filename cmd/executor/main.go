package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	pkgsimpleping "example.com/rbmq-demo/pkg/simpleping"
	pkgutils "example.com/rbmq-demo/pkg/utils"
)

func tryFlush(w http.ResponseWriter) {
	// The default HTTP/1.x and HTTP/2 ResponseWriter implementations support Flusher,
	// but ResponseWriter wrappers may not.
	// Handlers should always test for this ability at runtime.
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func main() {
	socketPath := "/var/run/myserver.sock"

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal("Error listening:", err)
	}
	defer listener.Close()
	log.Printf("Server listening on %s\n", socketPath)

	go func() {
		muxer := http.NewServeMux()
		muxer.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			respEncoder := json.NewEncoder(w)
			destination := r.URL.Query().Get("destination")
			pingCfg := &pkgsimpleping.PingConfiguration{
				Destination: destination,
				Count:       3,
				Timeout:     30 * time.Second,
				Interval:    1 * time.Second,
			}
			if count := r.URL.Query().Get("count"); count != "" {
				countInt, err := strconv.Atoi(count)
				if err != nil {
					respEncoder.Encode(pkgutils.ErrorResponse{Error: err.Error()})
					return
				}
				pingCfg.Count = countInt
			}
			if timeout := r.URL.Query().Get("timeout"); timeout != "" {
				timeoutInt, err := strconv.Atoi(timeout)
				if err != nil {
					respEncoder.Encode(pkgutils.ErrorResponse{Error: err.Error()})
					return
				}
				pingCfg.Timeout = time.Duration(timeoutInt) * time.Second
			}
			if interval := r.URL.Query().Get("interval"); interval != "" {
				intervalInt, err := strconv.Atoi(interval)
				if err != nil {
					respEncoder.Encode(pkgutils.ErrorResponse{Error: err.Error()})
					return
				}
				pingCfg.Interval = time.Duration(intervalInt) * time.Second
			}

			pinger := pkgsimpleping.NewSimplePinger(pingCfg)

			pingEvents := pinger.Ping(context.TODO())
			for ev := range pingEvents {
				err := respEncoder.Encode(ev)
				if err != nil {
					respEncoder.Encode(pkgutils.ErrorResponse{Error: err.Error()})
					tryFlush(w)
					break
				}
				tryFlush(w)
			}
		})

		server := http.Server{
			Handler: muxer,
		}

		if err := server.Serve(listener); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Fatal("Error serving:", err)
			}
			log.Println("Server exitted")
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal: %s", sig.String())
	log.Println("\nShutting down server...")
}
