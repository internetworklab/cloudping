package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	pkgauth "github.com/internetworklab/cloudping/pkg/auth"
	pkgconnreg "github.com/internetworklab/cloudping/pkg/connreg"
	pkgframing "github.com/internetworklab/cloudping/pkg/framing"
	quicGo "github.com/quic-go/quic-go"
)

type QUICHandler struct {
	Cr                *pkgconnreg.ConnRegistry
	Timeout           time.Duration
	Listener          *quicGo.Listener
	ShouldValidateJWT bool
	JWTValidator      pkgauth.JWTValidator
}

func (handler *QUICHandler) handleMessage(ctx context.Context, key string, stream *quicGo.Stream, msg []byte) error {
	cr := handler.Cr
	var payload pkgframing.MessagePayload
	err := json.Unmarshal(msg, &payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal message from %s: %v", key, err)
	}

	if payload.Register != nil {
		var registeredClaims *jwt.RegisteredClaims = nil
		if handler.ShouldValidateJWT {
			tokenString := payload.Register.Token
			if tokenString == nil {
				return fmt.Errorf("token string is nil")
			}
			registeredClaims, err = handler.JWTValidator.ParseToken(ctx, *tokenString)
			if err != nil {
				return fmt.Errorf("failed to validate JWT of peer %s: %v", key, err)
			}
		}

		if err := cr.Register(key, *payload.Register, registeredClaims); err != nil {
			return fmt.Errorf("failed to register connection from %s: %v", key, err)
		}
	}
	if payload.Echo != nil {
		if payload.Echo.Direction == pkgconnreg.EchoDirectionC2S {
			cr.UpdateHeartbeat(key)
			responsePayload := pkgframing.MessagePayload{
				Echo: &pkgconnreg.EchoPayload{
					Direction:       pkgconnreg.EchoDirectionS2C,
					CorrelationID:   payload.Echo.CorrelationID,
					ServerTimestamp: uint64(time.Now().UnixMilli()),
					Timestamp:       payload.Echo.Timestamp,
					SeqID:           payload.Echo.SeqID,
				},
			}
			err := json.NewEncoder(stream).Encode(responsePayload)
			if err != nil {
				return fmt.Errorf("failed to marshal response payload for sending to %s: %v", key, err)
			}
		}
	}
	if payload.AttributesAnnouncement != nil {
		cr.SetAttributes(key, payload.AttributesAnnouncement)
	}
	return nil
}

func (h *QUICHandler) Serve() {
	log.Printf("Serving QUIC listener on %s", h.Listener.Addr())
	for {
		conn, err := h.Listener.Accept(context.Background())
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
			return
		}
		log.Printf("Accepted QUIC connection: %p %s", conn, conn.RemoteAddr())
		key := uuid.New().String()
		go h.Handle(conn, key)
	}
}

func (h *QUICHandler) Handle(conn *quicGo.Conn, remoteKey string) {
	cr := h.Cr
	ctx := conn.Context()

	cr.OpenConnection(remoteKey, conn)
	log.Printf("Connection opened for %s, total connections: %d", remoteKey, cr.Count())

	defer func() {
		log.Printf("Closing QUIC connection: %s", remoteKey)
		err := conn.CloseWithError(quicGo.ApplicationErrorCode(quicGo.NoError), "Connection closed by server")
		if err != nil {
			log.Printf("Failed to close QUIC connection for %s: %v", remoteKey, err)
		}
		cr.CloseConnection(remoteKey)
		log.Printf("Connection closed for %s, remaining connections: %d", remoteKey, cr.Count())
	}()

	var gcTimer *time.Timer = nil
	if int64(h.Timeout) == 0 {
		panic("timeout is not set")
	}
	gcTimer = time.NewTimer(h.Timeout)
	defer func() {
		if gcTimer != nil {
			gcTimer.Stop()
			gcTimer = nil
		}
	}()

	connErrCh := make(chan error, 1)

	go func(conn *quicGo.Conn) {
		for {
			log.Printf("Accepting stream from connection: %s", remoteKey)
			stream, err := conn.AcceptStream(context.Background())
			if err != nil {
				connErrCh <- fmt.Errorf("failed to accept stream: %v", err)
				return
			}
			streamId := stream.StreamID()
			log.Printf("Accepted stream: %p %d from connection: %s", stream, streamId, remoteKey)

			go func(stream *quicGo.Stream) {
				defer stream.Close()
				defer log.Printf("Closing stream: %p %d of connection: %s", stream, streamId, remoteKey)
				scanner := bufio.NewScanner(stream)
				for scanner.Scan() {
					line := scanner.Bytes()
					if err := h.handleMessage(ctx, remoteKey, stream, line); err != nil {
						connErrCh <- fmt.Errorf("failed to handle text message from %s: %v", remoteKey, err)
						return
					}
					gcTimer.Reset(h.Timeout)
				}
				if err := scanner.Err(); err != nil {
					connErrCh <- fmt.Errorf("failed to scan stream of connection: %s: %v", remoteKey, err)
					return
				}
			}(stream)
		}
	}(conn)

	select {
	case <-gcTimer.C:
		log.Printf("Garbage collection timeout for %s, closing connection", remoteKey)
	case err := <-connErrCh:
		if err != nil {
			log.Printf("Connection error for %s: %v", remoteKey, err)
		}
	}
}
