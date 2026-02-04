package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	pkgconnreg "example.com/rbmq-demo/pkg/connreg"
	pkgframing "example.com/rbmq-demo/pkg/framing"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WebsocketHandler struct {
	upgrader *websocket.Upgrader
	cr       *pkgconnreg.ConnRegistry
	timeout  time.Duration
}

func NewWebsocketHandler(upgrader *websocket.Upgrader, cr *pkgconnreg.ConnRegistry, timeout time.Duration) *WebsocketHandler {
	return &WebsocketHandler{
		upgrader: upgrader,
		cr:       cr,
		timeout:  timeout,
	}
}

func (handler *WebsocketHandler) handleTextMessage(key string, conn *websocket.Conn, msg []byte) error {
	cr := handler.cr
	if cr == nil {
		return fmt.Errorf("connection registry is not set")
	}

	var payload pkgframing.MessagePayload
	err := json.Unmarshal(msg, &payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal message from %s: %v", key, err)
	}

	if payload.Register != nil {
		cr.Register(key, *payload.Register, nil)
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
			responseJSON, err := json.Marshal(responsePayload)
			if err != nil {
				return fmt.Errorf("failed to marshal response payload for %s: %v", key, err)
			}
			err = conn.WriteMessage(websocket.TextMessage, responseJSON)
			if err != nil {
				return fmt.Errorf("failed to write response message to %s: %v", key, err)
			}
		}
	}
	if payload.AttributesAnnouncement != nil {
		cr.SetAttributes(key, payload.AttributesAnnouncement)
	}
	return nil
}

func (h *WebsocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := h.upgrader
	cr := h.cr
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}

	remoteKey := uuid.New().String()
	cr.OpenConnection(remoteKey, nil)
	log.Printf("Connection opened for %s, total connections: %d", remoteKey, cr.Count())

	defer func() {
		log.Printf("Closing WebSocket connection: %s", remoteKey)
		err := conn.Close()
		if err != nil {
			log.Printf("Failed to close WebSocket connection for %s: %v", remoteKey, err)
		}
		cr.CloseConnection(remoteKey)
		log.Printf("Connection closed for %s, remaining connections: %d", remoteKey, cr.Count())
	}()

	var gcTimer *time.Timer = nil
	if int64(h.timeout) == 0 {
		panic("timeout is not set")
	}
	gcTimer = time.NewTimer(h.timeout)
	defer func() {
		if gcTimer != nil {
			gcTimer.Stop()
			gcTimer = nil
		}
	}()

	connErrCh := make(chan error)

	go func() {
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				connErrCh <- fmt.Errorf("failed to read message from %s: %v", remoteKey, err)
				break
			}

			switch msgType {
			case websocket.TextMessage:
				if err := h.handleTextMessage(remoteKey, conn, msg); err != nil {
					log.Printf("Failed to handle text message from %s: %v", remoteKey, err)
					continue
				}
				gcTimer.Reset(h.timeout)
			default:
				log.Printf("Received unknown message type from %s: %d", remoteKey, msgType)
			}
		}
	}()

	select {
	case <-gcTimer.C:
		log.Printf("Garbage collection timeout for %s, closing connection", remoteKey)
	case err := <-connErrCh:
		if err != nil {
			log.Printf("Connection error for %s: %v", remoteKey, err)
		}
	}
}
