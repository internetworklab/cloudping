package nodereg

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	pkgsafemap "github.com/internetworklab/cloudping/pkg/safemap"
	quicGo "github.com/quic-go/quic-go"
	quicHttp3 "github.com/quic-go/quic-go/http3"
)

const (
	AttributeKeyNodeName            = "NodeName"
	AttributeKeyPingCapability      = "CapabilityPing"
	AttributeKeyDNSProbeCapability  = "CapabilityDNSProbe"
	AttributeKeyHTTPProbeCapability = "CapabilityHTTPProbe"
	AttributeKeySupportQUICTunnel   = "SupportQUICTunnel"
	AttributeKeyHttpEndpoint        = "HttpEndpoint"
	AttributeKeyRespondRange        = "RespondRange"
	AttributeKeyExactLocation       = "ExactLocation"
	AttributeKeyCountryCode         = "CountryCode"
	AttributeKeyCityName            = "CityName"
	AttributeKeyASN                 = "ProviderASN"
	AttributeKeyISP                 = "ProviderName"
	AttributeKeyDN42ASN             = "DN42ProviderASN"
	AttributeKeyDN42ISP             = "DN42ProviderName"
	AttributeKeyDomainRespondRange  = "DomainRespondRange"
	AttributeKeySupportUDP          = "SupportUDP"
	AttributeKeySupportPMTU         = "SupportPMTU"
	AttributeKeySupportTCP          = "SupportTCP"
	AttributeKeyVersion             = "Version"
	AttributeKeyLivenessCheck       = "LivenessCheck"
)

type NodeRegistrationAgent struct {
	HTTPMuxer         http.Handler
	ClientCert        string
	ClientCertKey     string
	ServerAddress     string
	QUICServerAddress string
	NodeName          string
	CorrelationID     *string
	SeqID             *uint64
	TickInterval      time.Duration
	intialized        bool
	NodeAttributes    ConnectionAttributes
	LogEchoReplies    bool
	ServerName        string
	CustomCertPool    *x509.CertPool
	UseQUIC           bool
	Token             *string

	wsConn     *websocket.Conn
	quicConn   *quicGo.Conn
	quicStream *quicGo.Stream
}

func (agent *NodeRegistrationAgent) Init() error {
	if agent.ServerAddress == "" && agent.QUICServerAddress == "" {
		return fmt.Errorf("either server address or QUIC server address is required")
	}

	if agent.NodeName == "" {
		return fmt.Errorf("node name is required")
	}

	if agent.CorrelationID == nil {
		corrId := uuid.New().String()
		agent.CorrelationID = &corrId
		log.Printf("Using default correlation ID: %s", corrId)
	}

	if agent.SeqID == nil {
		seqId := uint64(0)
		agent.SeqID = &seqId
		log.Printf("Will start at sequence ID: %d", seqId)
	}

	log.Printf("Agent will use tick interval: %s", agent.TickInterval.String())

	agent.intialized = true
	return nil
}

func (agent *NodeRegistrationAgent) runReceiver() error {
	for {
		payload, err := agent.recvMsgPayload()
		if err != nil {
			return fmt.Errorf("failed to receive message from remote: %v", err)
		}

		if payload != nil && payload.Echo != nil &&
			payload.Echo.CorrelationID == *agent.CorrelationID &&
			payload.Echo.Direction == EchoDirectionS2C {

			rtt, onTrip, backTrip := payload.Echo.CalculateDelays(time.Now())
			if agent.LogEchoReplies {
				log.Printf("Received echo reply: Seq: %d, RTT: %d ms, On-trip: %d ms, Back-trip: %d ms", payload.Echo.SeqID, rtt.Milliseconds(), onTrip.Milliseconds(), backTrip.Milliseconds())
			}
		}
	}
}

// Connect and start the loop
func (agent *NodeRegistrationAgent) Run(ctx context.Context) chan error {
	errCh := make(chan error)
	go func() {
		errCh <- agent.doRun(ctx)
	}()
	return errCh
}

func (agent *NodeRegistrationAgent) getTLSConfig() (*tls.Config, error) {
	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to get system cert pool: %v", err)
	}

	tlsConfig := &tls.Config{
		RootCAs:    systemCertPool,
		ServerName: agent.ServerName,
	}
	if agent.CustomCertPool != nil {
		tlsConfig.RootCAs = agent.CustomCertPool
	}
	if agent.ClientCert != "" && agent.ClientCertKey != "" {
		cert, err := tls.LoadX509KeyPair(agent.ClientCert, agent.ClientCertKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %v", err)
		}
		if tlsConfig.Certificates == nil {
			tlsConfig.Certificates = make([]tls.Certificate, 0)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}
	return tlsConfig, nil
}

func (agent *NodeRegistrationAgent) connectWs(tlsConfig *tls.Config) (*websocket.Conn, error) {
	log.Printf("Agent %s started, connecting to WebSocket server %s", agent.NodeName, agent.ServerAddress)
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		TLSClientConfig:  tlsConfig,
	}

	c, _, err := dialer.Dial(agent.ServerAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %v", agent.ServerAddress, err)
	}

	log.Printf("Connected to WebSocket server %s: remote address: %s", agent.ServerAddress, c.RemoteAddr())
	return c, nil
}

func (agent *NodeRegistrationAgent) getRegisterPayload() MessagePayload {
	registerPayload := RegisterPayload{
		NodeName: agent.NodeName,
		Token:    agent.Token,
	}
	registerMsg := MessagePayload{
		Register: &registerPayload,
	}
	if agent.NodeAttributes != nil {
		registerMsg.AttributesAnnouncement = &AttributesAnnouncementPayload{
			Attributes: agent.NodeAttributes,
		}
	}
	return registerMsg
}

func (agent *NodeRegistrationAgent) getTickMsg() (MessagePayload, uint64) {
	msg := MessagePayload{
		Echo: &EchoPayload{
			Direction:     EchoDirectionC2S,
			CorrelationID: *agent.CorrelationID,
			Timestamp:     uint64(time.Now().UnixMilli()),
			SeqID:         *agent.SeqID,
		},
	}
	nextSeq := *agent.SeqID + 1
	return msg, nextSeq
}

func (agent *NodeRegistrationAgent) doRun(ctx context.Context) error {
	if !agent.intialized {
		return fmt.Errorf("agent not initialized")
	}

	errCh := make(chan error)

	go func() {
		registerMsg := agent.getRegisterPayload()

		defer close(errCh)

		tlsConfig, err := agent.getTLSConfig()
		if err != nil {
			errCh <- fmt.Errorf("failed to get TLS config: %v", err)
			return
		}

		if agent.UseQUIC {
			if !slices.Contains(tlsConfig.NextProtos, "h3") {
				if tlsConfig.NextProtos == nil {
					tlsConfig.NextProtos = make([]string, 0)
				}
				tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h3")
			}

			log.Printf("Dialing QUIC address %s", agent.QUICServerAddress)
			quicConn, err := quicGo.DialAddr(ctx, agent.QUICServerAddress, tlsConfig, nil)
			if err != nil {
				errCh <- fmt.Errorf("failed to dial QUIC address %s: %v", agent.QUICServerAddress, err)
				return
			}
			agent.quicConn = quicConn
			quicRAddr := quicConn.RemoteAddr()
			log.Printf("Dialed QUIC address: %s,", quicRAddr)

			stream, err := agent.quicConn.OpenStreamSync(ctx)
			if err != nil {
				errCh <- fmt.Errorf("failed to open QUIC stream for %s: %v", quicRAddr, err)
				return
			}
			agent.quicStream = stream

			server := &quicHttp3.Server{
				Handler: agent.HTTPMuxer,
			}
			rawServerConn, err := server.NewRawServerConn(agent.quicConn)
			if err != nil {
				log.Printf("failed to create raw server connection from quic %s: %v", quicRAddr, err)
				return
			}
			log.Printf("Created raw server connection from quic %s", quicRAddr)

			go func() {
				defer log.Printf("QUIC proxy is exitting")
				for {
					stream, err := agent.quicConn.AcceptStream(ctx)
					if err != nil {
						log.Printf("failed to accept QUIC stream from conn %s: %v", quicRAddr, err)
						return
					}
					log.Printf("Accepted QUIC stream %d from conn %s", stream.StreamID(), quicRAddr)
					go func(stream *quicGo.Stream) {
						defer stream.Close()
						defer log.Printf("QUIC stream %d of conn %s exitting", stream.StreamID(), quicRAddr)

						log.Printf("Handling request stream %d of conn %s", stream.StreamID(), quicRAddr)
						rawServerConn.HandleRequestStream(stream)
					}(stream)
				}
			}()
		} else {
			c, err := agent.connectWs(tlsConfig)
			if err != nil {
				errCh <- fmt.Errorf("failed to connect to WS: %v", err)
				return
			}
			agent.wsConn = c
		}

		receiverExit := make(chan error)
		go func() {
			receiverExit <- agent.runReceiver()
		}()

		ticker := time.NewTicker(agent.TickInterval)
		defer ticker.Stop()

		log.Printf("Sending register message")
		if err := agent.sendMsgPayload(&registerMsg); err != nil {
			errCh <- fmt.Errorf("failed to send register message: %v", err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case receiverErr := <-receiverExit:
				var err error
				if receiverErr != nil {
					err = fmt.Errorf("receiver exited with error: %v", receiverErr)
				}
				errCh <- err
				return
			case <-ticker.C:
				var msg MessagePayload
				msg, *agent.SeqID = agent.getTickMsg()
				if err := agent.sendMsgPayload(&msg); err != nil {
					errCh <- fmt.Errorf("failed to send echo message: %v", err)
					return
				}
			}
		}
	}()

	return <-errCh
}

func (agent *NodeRegistrationAgent) sendMsgPayload(payload *MessagePayload) error {
	if agent.UseQUIC {
		if agent.quicStream == nil {
			panic("quic stream shouldn't be nil when using QUIC")
		}
		return json.NewEncoder(agent.quicStream).Encode(payload)
	}

	if agent.wsConn == nil {
		panic("ws conn shouldn't be nil when using WebSocket")
	}

	return agent.wsConn.WriteJSON(payload)
}

func (agent *NodeRegistrationAgent) recvMsgPayload() (*MessagePayload, error) {
	var payload MessagePayload
	if agent.UseQUIC {
		if agent.quicStream == nil {
			panic("quic stream shouldn't be nil when using QUIC")
		}

		err := json.NewDecoder(agent.quicStream).Decode(&payload)
		if err != nil {
			return nil, fmt.Errorf("failed to read or decode message from QUIC stream: %v", err)
		}
		return &payload, nil
	}

	if agent.wsConn == nil {
		panic("ws conn shouldn't be nil when using WebSocket")
	}

	err := agent.wsConn.ReadJSON(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to read or decode message from WebSocket: %v", err)
	}

	return &payload, nil
}

type MessagePayload struct {
	Register               *RegisterPayload               `json:"register,omitempty"`
	Echo                   *EchoPayload                   `json:"echo,omitempty"`
	AttributesAnnouncement *AttributesAnnouncementPayload `json:"attributes_announcement,omitempty"`
}

type RegisterPayload struct {
	NodeName string  `json:"node_name"`
	Token    *string `json:"token,omitempty"`
}

type EchoDirection string

const (
	EchoDirectionC2S EchoDirection = "ping"
	EchoDirectionS2C EchoDirection = "pong"
)

type EchoPayload struct {
	Direction       EchoDirection `json:"direction"`
	CorrelationID   string        `json:"correlation_id"`
	ServerTimestamp uint64        `json:"server_timestamp"`
	Timestamp       uint64        `json:"timestamp"`
	SeqID           uint64        `json:"seq_id"`
}

type AttributesAnnouncementPayload struct {
	Attributes  ConnectionAttributes `json:"attributes,omitempty"`
	Withdrawals []string             `json:"withdrawals,omitempty"`
}

func (echopayload *EchoPayload) CalculateDelays(now time.Time) (rtt time.Duration, onTrip time.Duration, backTrip time.Duration) {
	rtt = now.Sub(time.UnixMilli(int64(echopayload.Timestamp)))
	onTrip = time.UnixMilli(int64(echopayload.ServerTimestamp)).Sub(time.UnixMilli(int64(echopayload.Timestamp)))
	backTrip = now.Sub(time.UnixMilli(int64(echopayload.ServerTimestamp)))

	return rtt, onTrip, backTrip
}

type ConnectionAttributes map[string]string

type AuthenticationType string

const (
	AuthenticationTypeJWT  AuthenticationType = "jwt"
	AuthenticationTypeMTLS AuthenticationType = "mtls"
)

type ConnRegistryData struct {
	NodeName       *string               `json:"node_name,omitempty"`
	ConnectedAt    uint64                `json:"connected_at"`
	RegisteredAt   *uint64               `json:"registered_at,omitempty"`
	LastHeartbeat  *uint64               `json:"last_heartbeat,omitempty"`
	Attributes     ConnectionAttributes  `json:"attributes,omitempty"`
	QUICConn       *quicGo.Conn          `json:"-"`
	Claims         *jwt.RegisteredClaims `json:"-"`
	Authentication AuthenticationType    `json:"authentication"`
}

func (regData *ConnRegistryData) Clone() *ConnRegistryData {
	return cloneConnRegistryData(regData).(*ConnRegistryData)
}

func cloneConnRegistryData(dataany interface{}) interface{} {
	data, ok := dataany.(*ConnRegistryData)
	if !ok {
		panic(fmt.Errorf("failed to convert dataany to *ConnRegistryData"))
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to marshal connection registry data: %v", err)
		panic(err)
	}

	var cloned *ConnRegistryData
	err = json.Unmarshal(bytes, &cloned)
	if err != nil {
		panic(err)
	}
	if data.QUICConn != nil {
		cloned.QUICConn = data.QUICConn
	}
	return cloned
}

type ConnRegistry struct {
	datastore pkgsafemap.DataStore
}

func (cr *ConnRegistry) OpenConnection(key string, quicConn *quicGo.Conn) {
	now := uint64(time.Now().Unix())
	connRegData := &ConnRegistryData{
		ConnectedAt: now,
		Attributes:  make(ConnectionAttributes),
		QUICConn:    quicConn,
	}
	cr.datastore.Set(key, connRegData)
}

func (cr *ConnRegistry) CloseConnection(key string) {
	cr.datastore.Delete(key)
}

func (cr *ConnRegistry) Register(key string, payload RegisterPayload, claims *jwt.RegisteredClaims) error {
	log.Printf("Registering connection from %s, node name: %s", key, payload.NodeName)

	_, found := cr.datastore.Get(key, func(valany interface{}) error {
		entry := valany.(*ConnRegistryData)
		now := uint64(time.Now().Unix())
		if entry == nil {
			return fmt.Errorf("connection from %s not found in registry", key)
		}
		entry.NodeName = &payload.NodeName
		entry.RegisteredAt = &now

		if claims != nil {
			entry.Claims = claims
			entry.Authentication = AuthenticationTypeJWT
		} else {
			entry.Authentication = AuthenticationTypeMTLS
		}

		return nil
	})

	if !found {
		return fmt.Errorf("connection from %s not found in registry", key)
	}
	return nil
}

func (cr *ConnRegistry) UpdateHeartbeat(key string) error {
	_, found := cr.datastore.Get(key, func(valany interface{}) error {
		entry := valany.(*ConnRegistryData)
		now := uint64(time.Now().Unix())
		entry.LastHeartbeat = &now

		return nil
	})

	if !found {
		return fmt.Errorf("connection from %s not found in registry", key)
	}
	return nil
}

func (cr *ConnRegistry) SetAttributes(connkey string, announcement *AttributesAnnouncementPayload) error {
	_, found := cr.datastore.Get(connkey, func(valany interface{}) error {
		entry := valany.(*ConnRegistryData)
		attrs := make(ConnectionAttributes)
		for k, v := range entry.Attributes {
			attrs[k] = v
		}
		for _, withdrawal := range announcement.Withdrawals {
			delete(attrs, withdrawal)
		}
		for k, v := range announcement.Attributes {
			attrs[k] = v
		}
		entry.Attributes = attrs
		return nil
	})
	if !found {
		return fmt.Errorf("connection from %s not found in registry", connkey)
	}
	return nil
}

type NodeTesterFunc func(nodeKey string, nodeData *ConnRegistryData) bool

func (cr *ConnRegistry) DumpFunc(tester NodeTesterFunc) map[string]*ConnRegistryData {
	dummped := cr.datastore.Dump(cloneConnRegistryData)
	result := make(map[string]*ConnRegistryData)
	for k, v := range dummped {
		if v == nil {
			continue
		}
		nodeData := v.(*ConnRegistryData)
		if tester != nil && !tester(k, nodeData) {
			continue
		}
		result[k] = nodeData
	}
	return result
}

func IsLiveNode(_ string, nodeData *ConnRegistryData) bool {
	if nodeData != nil {
		if attributes := nodeData.Attributes; attributes != nil {
			if val, hit := attributes[AttributeKeyLivenessCheck]; hit && val == "true" {
				return true
			}
		}
	}
	return false
}

func (cr *ConnRegistry) DumpLive() map[string]*ConnRegistryData {
	return cr.DumpFunc(IsLiveNode)
}

func (cr *ConnRegistry) Dump() map[string]*ConnRegistryData {
	return cr.DumpFunc(nil)
}

func (cr *ConnRegistry) Count() int {
	return cr.datastore.Len()
}

func NewConnRegistry(datastore pkgsafemap.DataStore) *ConnRegistry {
	connReg := &ConnRegistry{
		datastore: datastore,
	}
	return connReg
}

// If all matches, return true, otherwise return false
func (regData *ConnRegistryData) TestAgainstAttributes(expected ConnectionAttributes) (allMatch bool) {
	allMatch = true
	for k, v := range expected {
		actual, ok := regData.Attributes[k]
		if !ok {
			allMatch = false
			break
		}
		if actual != v {
			allMatch = false
			break
		}
	}
	return allMatch
}

func (cr *ConnRegistry) SearchByAttributes(expected ConnectionAttributes) (data *ConnRegistryData, err error) {
	err = cr.datastore.Walk(func(key string, value interface{}) (keepgoing bool, err error) {
		entry, ok := value.(*ConnRegistryData)
		if !ok {
			return false, fmt.Errorf("failed to convert value to *ConnRegistryData")
		}

		keepgoing = !entry.TestAgainstAttributes(expected)
		if !keepgoing {
			data = cloneConnRegistryData(entry).(*ConnRegistryData)
		}

		return keepgoing, nil
	})

	return data, err
}

func (cr *ConnRegistry) Shutdown(ctx context.Context) error {
	return nil
}
