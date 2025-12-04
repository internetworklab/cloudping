package raw

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// Package consists of types and functions for sending, as well as receiving, ICMP packets.

type RawBinary []byte

func (rb RawBinary) MarshalJSON() ([]byte, error) {
	return []byte("\"" + base64.StdEncoding.EncodeToString(rb) + "\""), nil
}

type WrappedPacketType string

const (
	PktICMPRequest  WrappedPacketType = "icmp_request"
	PktICMPResponse WrappedPacketType = "icmp_response"
)

type WrappedPacket struct {
	Id   int               `json:"id"`
	Seq  int               `json:"seq"`
	Src  string            `json:"src"`
	Dst  string            `json:"dst"`
	Type WrappedPacketType `json:"type"`

	// todo: implement raw ip packet later
	TTL *int `json:"ttl,omitempty"`

	// Note: the Data field is completely optional, and might be omitted for generality.
	Data RawBinary `json:"data,omitempty"`
}

type ICMPTransceiverConfig struct {
	Id          int
	Destination string
	InitialSeq  int

	// Buffer size for receiving and parsing single packet
	BufSize int

	// Custom resolver might be provided by the user
	CustomResolver *net.Resolver
}

type ICMPTransceiver struct {
	config ICMPTransceiverConfig

	conn  *icmp.PacketConn
	conn6 *icmp.PacketConn

	// Direction of In: User -> Here
	// the value is the seq
	In chan int

	// Direction of Out: Here -> User
	Out chan *WrappedPacket

	// The actual pinging destination IP address
	DestinationIP net.IP
}

func NewICMPTransceiver(config ICMPTransceiverConfig) (*ICMPTransceiver, error) {

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on packet:icmp: %v", err)
	}

	conn6, err := icmp.ListenPacket("ip6:icmp", "::")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on packet:icmp: %v", err)
	}

	return &ICMPTransceiver{
		config: config,
		In:     make(chan int),
		Out:    make(chan *WrappedPacket),
		conn:   conn,
		conn6:  conn6,
	}, nil
}

func (icmpTR *ICMPTransceiver) readICMPPackets(conn *icmp.PacketConn, protocol int) error {
	receivBuf := make([]byte, icmpTR.config.BufSize)
	n, peer, err := icmpTR.conn.ReadFrom(receivBuf)
	if err != nil {
		return fmt.Errorf("failed to read from connection: %v", err)
	}
	log.Printf("read %d bytes from %s", n, peer.String())
	receivMsg, err := icmp.ParseMessage(protocol, receivBuf[:n])
	if err != nil {
		return fmt.Errorf("failed to parse icmp message: %v", err)
	}
	if icmpEcho, ok := receivMsg.Body.(*icmp.Echo); ok && icmpEcho.ID == icmpTR.config.Id {
		wrappedPkt := &WrappedPacket{
			Id:   icmpEcho.ID,
			Seq:  icmpEcho.Seq,
			Src:  peer.String(),
			Dst:  icmpTR.config.Destination,
			Type: PktICMPResponse,
			TTL:  nil,
			Data: RawBinary(receivBuf[:n]),
		}
		icmpTR.Out <- wrappedPkt
	}
	return nil
}

func checkInetFamilySupportness() (v4 bool, v6 bool, err error) {
	v4 = false
	v6 = false

	ifaces, err := net.Interfaces()
	if err != nil {
		return v4, v6, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("failed to get addrs of interface %s: %v", iface.Name, err)
			continue
		}
		for _, addr := range addrs {
			if addr.Network() == "ip4" {
				v4 = true
				continue
			}
			if addr.Network() == "ip6" {
				v6 = true
				continue
			}
		}
	}

	return v4, v6, nil
}

func (icmpTR *ICMPTransceiver) Run(ctx context.Context) error {

	v4Sup, v6Sup, err := checkInetFamilySupportness()
	if err != nil {
		return fmt.Errorf("can't determine internet family supportness: %v", err)
	}

	if !v4Sup && !v6Sup {
		return fmt.Errorf("no internet family supportness found")
	}

	// a nil resolver is a default resolver
	var resolver *net.Resolver = icmpTR.config.CustomResolver
	ipCandidates := make([]net.IP, 0)
	if v6Sup {
		v6IPs, err := resolver.LookupIP(ctx, "ip6", icmpTR.config.Destination)
		if err == nil {
			ipCandidates = append(ipCandidates, v6IPs...)
		}
	}
	if v4Sup {
		v4IPs, err := resolver.LookupIP(ctx, "ip4", icmpTR.config.Destination)
		if err == nil {
			ipCandidates = append(ipCandidates, v4IPs...)
		}
	}
	if len(ipCandidates) == 0 {
		return fmt.Errorf("no ip candidates found for destination %s", icmpTR.config.Destination)
	}

	icmpTR.DestinationIP = ipCandidates[0]
	log.Printf("will send pings to %s", icmpTR.DestinationIP.String())

	var writePacketConn *icmp.PacketConn
	var writeICMPType icmp.Type

	if icmpTR.DestinationIP.To4() != nil {
		// only start one packet receiver based on detected inet family
		writePacketConn = icmpTR.conn
		writeICMPType = ipv4.ICMPTypeEcho
		go func() {
			log.Printf("starting v4 icmp parser")
			for {
				select {
				case <-ctx.Done():
					log.Printf("exitting v4 icmp parser")
					return
				default:
					err := icmpTR.readICMPPackets(icmpTR.conn, ipv4.ICMPTypeEchoReply.Protocol())
					if err != nil {
						log.Printf("failed to read v4 icmp packets: %v", err)
					}
				}
			}
		}()
	} else {
		writePacketConn = icmpTR.conn6
		writeICMPType = ipv6.ICMPTypeEchoRequest
		go func() {
			log.Printf("starting v6 icmp parser")
			for {
				select {
				case <-ctx.Done():
					log.Printf("exitting v6 icmp parser")
					return
				default:
					err := icmpTR.readICMPPackets(icmpTR.conn6, ipv6.ICMPTypeEchoReply.Protocol())
					if err != nil {
						log.Printf("failed to read v6 icmp packets: %v", err)
					}
				}
			}
		}()
	}

	log.Printf("sending ICMP type: %v", writeICMPType)

	go func() {
		for {
			select {
			case <-ctx.Done():
				icmpTR.doCleanUp()
				return
			case seq := <-icmpTR.In:
				writeMsg := icmp.Message{
					Type: writeICMPType,
					Body: &icmp.Echo{
						ID:  icmpTR.config.Id,
						Seq: seq,
					},
				}
				writeBuff, err := writeMsg.Marshal(nil)
				if err != nil {
					log.Printf("failed to marshal icmp message: %v", err)
					continue
				}
				if _, err := writePacketConn.WriteTo(writeBuff, &net.IPAddr{IP: icmpTR.DestinationIP}); err != nil {
					log.Printf("failed to write to connection: %v", err)
				}
			}
		}
	}()
	return nil
}

func (icmpTR *ICMPTransceiver) doCleanUp() {
	if err := icmpTR.conn.Close(); err != nil {
		log.Printf("failed to close connection: %v", err)
	}
	if err := icmpTR.conn6.Close(); err != nil {
		log.Printf("failed to close connection: %v", err)
	}
}
