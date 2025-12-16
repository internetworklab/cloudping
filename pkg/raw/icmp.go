package raw

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const ipv4HeaderLen int = 20
const ipv6HeaderLen int = 40
const headerSizeICMP int = 8
const protocolNumberICMPv4 int = 1
const protocolNumberICMPv6 int = 58

type GeneralICMPTransceiver interface {
	GetSender() chan<- ICMPSendRequest
	GetReceiver() chan<- chan ICMPReceiveReply
}

type ICMP4TransceiverConfig struct {
	// ICMP ID to use
	ID int
}

type ICMPSendRequest struct {
	Dst  net.IPAddr
	Seq  int
	TTL  int
	Data []byte
}

type ICMPReceiveReply struct {
	ID   int
	Size int
	Seq  int
	TTL  int

	// the Src of the icmp echo reply, in string
	Peer string

	PeerRaw   net.Addr    `json:"-"`
	PeerRawIP *net.IPAddr `json:"-"`

	PeerRDNS []string

	ReceivedAt time.Time

	ICMPTypeV4 *ipv4.ICMPType
	ICMPTypeV6 *ipv6.ICMPType

	SetMTUTo            *int
	ShrinkICMPPayloadTo *int `json:"-"`
}

func (icmpReply *ICMPReceiveReply) ResolveRDNS(ctx context.Context, resolver *net.Resolver) (*ICMPReceiveReply, error) {
	clonedICMPReply := new(ICMPReceiveReply)
	*clonedICMPReply = *icmpReply
	ptrAnswers, err := resolver.LookupAddr(ctx, clonedICMPReply.Peer)
	if err == nil {
		clonedICMPReply.PeerRDNS = ptrAnswers
	}
	return clonedICMPReply, err
}

type ICMP4Transceiver struct {
	id             int
	packetConn     net.PacketConn
	ipv4PacketConn *ipv4.PacketConn

	// User send requests to here, we retrieve the request,
	// then we translate it to the wire format.
	SendC chan ICMPSendRequest

	// ICMP replies
	ReceiveC chan chan ICMPReceiveReply
}

func NewICMP4Transceiver(config ICMP4TransceiverConfig) (*ICMP4Transceiver, error) {

	tracer := &ICMP4Transceiver{
		id:       config.ID,
		SendC:    make(chan ICMPSendRequest),
		ReceiveC: make(chan chan ICMPReceiveReply),
	}

	return tracer, nil
}

func getMaximumMTU() int {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	maximumMTU := -1
	for _, iface := range ifaces {
		if iface.MTU > maximumMTU {
			maximumMTU = iface.MTU
		}
	}
	if maximumMTU == -1 {
		panic("can't determine maximum MTU")
	}
	return maximumMTU
}

func (icmp4tr *ICMP4Transceiver) Run(ctx context.Context) error {
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return fmt.Errorf("failed to listen on packet:icmp: %v", err)
	}
	go func() {
		defer conn.Close()

		icmp4tr.packetConn = conn
		icmp4tr.ipv4PacketConn = ipv4.NewPacketConn(conn)

		if err := icmp4tr.ipv4PacketConn.SetControlMessage(ipv4.FlagTTL|ipv4.FlagSrc|ipv4.FlagDst|ipv4.FlagInterface, true); err != nil {
			log.Fatal(err)
		}

		wm := icmp.Message{
			Type: ipv4.ICMPTypeEcho, Code: 0,
			Body: &icmp.Echo{
				ID:   icmp4tr.id,
				Data: nil,
			},
		}

		bufSize := getMaximumMTU()
		rb := make([]byte, bufSize)

		// launch receiving goroutine
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case replysSubCh := <-icmp4tr.ReceiveC:
					for {
						nBytes, ctrlMsg, peerAddr, err := icmp4tr.ipv4PacketConn.ReadFrom(rb)
						if err != nil {
							if err, ok := err.(net.Error); ok && err.Timeout() {
								continue
							}
							log.Printf("failed to read from connection: %v", err)
							break
						}

						// 1 is the ICMPv4 protocol number in IP header's protocol field
						receiveMsg, err := icmp.ParseMessage(1, rb[:nBytes])
						if err != nil {
							log.Fatalf("failed to parse icmp message: %v", err)
						}

						receivedAt := time.Now()
						replyObject := ICMPReceiveReply{
							ID:         icmp4tr.id,
							Size:       nBytes,
							ReceivedAt: receivedAt,
							Peer:       peerAddr.String(),
							TTL:        ctrlMsg.TTL,
							Seq:        -1, // if can't determine, use -1
							PeerRaw:    peerAddr,
						}
						if ipAddr, ok := peerAddr.(*net.IPAddr); ok {
							replyObject.PeerRawIP = ipAddr
						}

						var ty ipv4.ICMPType

						switch receiveMsg.Type {
						case ipv4.ICMPTypeDestinationUnreachable:
							ty = ipv4.ICMPTypeDestinationUnreachable
							dstUnreachBody, ok := receiveMsg.Body.(*icmp.DstUnreach)
							if !ok {
								log.Printf("Invalid ICMP Destination Unreachable body: %+v", receiveMsg)
								continue
							}

							// Check if it's Code 4 (Fragmentation Needed / Packet Too Big)
							// The MTU is stored in bytes 6-7 of the ICMP message header (big-endian)
							if receiveMsg.Code == 4 && nBytes >= 8 {
								mtu := int(rb[6])<<8 | int(rb[7])
								replyObject.SetMTUTo = &mtu
							}

							// Extract original ICMP message from Destination Unreachable body
							if len(dstUnreachBody.Data) < ipv4HeaderLen+headerSizeICMP {
								log.Printf("Invalid ICMP Destination Unreachable body: %+v", receiveMsg)
								continue
							}

							// Skip IP header (minimum 20 bytes, but check IHL field for actual length)
							ipHeaderLen := int(dstUnreachBody.Data[0]&0x0F) * 4
							if ipHeaderLen < 20 {
								ipHeaderLen = 20
							}
							if replyObject.SetMTUTo != nil {
								newMTU := *replyObject.SetMTUTo
								shrinkTo := newMTU - ipHeaderLen - headerSizeICMP
								if shrinkTo < 0 {
									shrinkTo = 0
								}
								replyObject.ShrinkICMPPayloadTo = &shrinkTo
							}

							if len(dstUnreachBody.Data) < ipHeaderLen+headerSizeICMP {
								log.Printf("Invalid ICMP Destination Unreachable message: %+v", receiveMsg)
								continue
							}

							// Parse the original ICMP message (protocol 1 for ICMP)
							originalICMPData := dstUnreachBody.Data[ipHeaderLen:]
							originalICMPMsg, err := icmp.ParseMessage(protocolNumberICMPv4, originalICMPData)
							if err != nil {
								log.Printf("Invalid ICMP Destination Unreachable message: %+v error: %+v", receiveMsg, err)
								continue
							}

							echoBody, ok := originalICMPMsg.Body.(*icmp.Echo)
							if !ok {
								log.Printf("Invalid ICMP Destination Unreachable message: %+v", receiveMsg)
								continue
							}

							replyObject.Seq = echoBody.Seq
							replyObject.ID = echoBody.ID
						case ipv4.ICMPTypeTimeExceeded:
							ty = ipv4.ICMPTypeTimeExceeded

							// Extract original ICMP message from TimeExceeded body
							timeExceededBody, ok := receiveMsg.Body.(*icmp.TimeExceeded)
							if !ok || len(timeExceededBody.Data) < ipv4HeaderLen+headerSizeICMP {
								log.Printf("Invalid ICMP Time-Exceeded body: %+v", receiveMsg)
								continue
							}

							// Skip IP header (minimum 20 bytes, but check IHL field for actual length)
							ipHeaderLen := int(timeExceededBody.Data[0]&0x0F) * 4
							if ipHeaderLen < 20 {
								ipHeaderLen = 20
							}

							if len(timeExceededBody.Data) < ipHeaderLen+headerSizeICMP {
								log.Printf("Invalid ICMP Time-Exceeded message: %+v", receiveMsg)
								continue
							}

							// Parse the original ICMP message (protocol 1 for ICMP)
							originalICMPData := timeExceededBody.Data[ipHeaderLen:]
							originalICMPMsg, err := icmp.ParseMessage(protocolNumberICMPv4, originalICMPData)
							if err != nil {
								log.Printf("Invalid ICMP Time-Exceeded message: %+v error: %+v", receiveMsg, err)
								continue
							}

							echoBody, ok := originalICMPMsg.Body.(*icmp.Echo)
							if !ok {
								log.Printf("Invalid ICMP Time-Exceeded message: %+v", receiveMsg)
								continue
							}

							replyObject.Seq = echoBody.Seq
							replyObject.ID = echoBody.ID
						case ipv4.ICMPTypeEchoReply:
							ty = ipv4.ICMPTypeEchoReply
							icmpBody, ok := receiveMsg.Body.(*icmp.Echo)
							if !ok {
								log.Printf("failed to parse icmp body: %+v", receiveMsg)
								continue
							}
							replyObject.Seq = icmpBody.Seq
							replyObject.ID = icmpBody.ID
						default:
							log.Printf("unknown ICMP message: %+v", receiveMsg)
							continue
						}
						if replyObject.ID != icmp4tr.id {
							// this packet is not intended for us, silently ignore it
							continue
						}

						replyObject.ICMPTypeV4 = &ty
						replysSubCh <- replyObject
						break
					}
				}
			}
		}()

		// launch sending goroutine
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case req, ok := <-icmp4tr.SendC:
					if !ok {
						return
					}

					wm.Body.(*icmp.Echo).Seq = req.Seq
					wm.Body.(*icmp.Echo).Data = req.Data
					wb, err := wm.Marshal(nil)
					if err != nil {
						log.Fatalf("failed to marshal icmp message: %v", err)
					}

					if err := icmp4tr.ipv4PacketConn.SetTTL(req.TTL); err != nil {
						log.Fatalf("failed to set TTL to %v: %v, req: %+v", req.TTL, err, req)
					}

					dst := req.Dst
					if _, err := icmp4tr.ipv4PacketConn.WriteTo(wb, nil, &dst); err != nil {
						log.Fatalf("failed to write to connection: %v", err)
					}
				}
			}
		}()

		<-ctx.Done()
	}()

	return nil
}

func (icmp4tr *ICMP4Transceiver) GetSender() chan<- ICMPSendRequest {
	return icmp4tr.SendC
}

func (icmp4tr *ICMP4Transceiver) GetReceiver() chan<- chan ICMPReceiveReply {
	return icmp4tr.ReceiveC
}

type ICMP6TransceiverConfig struct {
	// ICMP ID to use
	ID int
}

type ICMP6Transceiver struct {
	id             int
	packetConn     net.PacketConn
	ipv6PacketConn *ipv6.PacketConn

	// User send requests to here, we retrieve the request,
	// then we translate it to the wire format.
	SendC chan ICMPSendRequest

	// ICMP replies
	ReceiveC chan chan ICMPReceiveReply
}

func NewICMP6Transceiver(config ICMP6TransceiverConfig) (*ICMP6Transceiver, error) {

	tracer := &ICMP6Transceiver{
		id:       config.ID,
		SendC:    make(chan ICMPSendRequest),
		ReceiveC: make(chan chan ICMPReceiveReply),
	}

	return tracer, nil
}

func (icmp6tr *ICMP6Transceiver) Run(ctx context.Context) error {
	conn, err := net.ListenPacket("ip6:58", "::")
	if err != nil {
		return fmt.Errorf("failed to listen on packet:ip6-icmp: %v", err)
	}
	go func() {
		defer conn.Close()

		icmp6tr.packetConn = conn
		icmp6tr.ipv6PacketConn = ipv6.NewPacketConn(conn)

		if err := icmp6tr.ipv6PacketConn.SetControlMessage(ipv6.FlagHopLimit|ipv6.FlagSrc|ipv6.FlagDst|ipv6.FlagInterface, true); err != nil {
			log.Fatal(err)
		}

		wm := icmp.Message{
			Type: ipv6.ICMPTypeEchoRequest, Code: 0,
			Body: &icmp.Echo{
				ID:   icmp6tr.id,
				Data: nil,
			},
		}

		var f ipv6.ICMPFilter
		f.SetAll(true)
		f.Accept(ipv6.ICMPTypeTimeExceeded)
		f.Accept(ipv6.ICMPTypeEchoReply)
		f.Accept(ipv6.ICMPTypePacketTooBig)
		if err := icmp6tr.ipv6PacketConn.SetICMPFilter(&f); err != nil {
			log.Fatal(err)
		}

		var wcm ipv6.ControlMessage
		bufSize := getMaximumMTU()
		rb := make([]byte, bufSize)

		// launch receiving goroutine
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case replySubCh := <-icmp6tr.ReceiveC:
					for {
						nBytes, ctrlMsg, peerAddr, err := icmp6tr.ipv6PacketConn.ReadFrom(rb)
						if err != nil {
							if err, ok := err.(net.Error); ok && err.Timeout() {
								log.Printf("timeout reading from connection, skipping")
								continue
							}
							log.Printf("failed to read from connection: %v", err)
							break
						}

						// 58 is the ICMPv6 protocol number in IPv6 header's protocol field
						receiveMsg, err := icmp.ParseMessage(protocolNumberICMPv6, rb[:nBytes])
						if err != nil {
							log.Fatalf("failed to parse icmp message: %v", err)
						}

						receivedAt := time.Now()
						replyObject := ICMPReceiveReply{
							ID:         icmp6tr.id,
							Size:       nBytes,
							ReceivedAt: receivedAt,
							Peer:       peerAddr.String(),
							TTL:        ctrlMsg.HopLimit,
							Seq:        -1, // if can't determine, use -1
						}

						var ty ipv6.ICMPType

						switch receiveMsg.Type {
						case ipv6.ICMPTypePacketTooBig:
							ty = ipv6.ICMPTypePacketTooBig
							packetTooBigBody, ok := receiveMsg.Body.(*icmp.PacketTooBig)
							if !ok {
								log.Printf("Invalid ICMP Packet Too Big body: %+v", receiveMsg)
								continue
							}

							// Extract MTU from PacketTooBig message
							mtu := packetTooBigBody.MTU
							replyObject.SetMTUTo = &mtu
							shrinkTo := mtu - ipv6HeaderLen - headerSizeICMP
							if shrinkTo < 0 {
								shrinkTo = 0
							}
							replyObject.ShrinkICMPPayloadTo = &shrinkTo

							// Extract original ICMP message from PacketTooBig body
							if len(packetTooBigBody.Data) < ipv6HeaderLen+headerSizeICMP {
								log.Printf("Invalid ICMP Packet Too Big body: %+v", receiveMsg)
								continue
							}

							// Skip IPv6 header (fixed 40 bytes)
							originalICMPData := packetTooBigBody.Data[ipv6HeaderLen:]
							originalICMPMsg, err := icmp.ParseMessage(protocolNumberICMPv6, originalICMPData)
							if err != nil {
								log.Printf("Invalid ICMP Packet Too Big message: %+v error: %+v", receiveMsg, err)
								continue
							}

							echoBody, ok := originalICMPMsg.Body.(*icmp.Echo)
							if !ok {
								log.Printf("Invalid ICMP Packet Too Big message: %+v", receiveMsg)
								continue
							}

							replyObject.Seq = echoBody.Seq
							replyObject.ID = echoBody.ID
						case ipv6.ICMPTypeTimeExceeded:
							ty = ipv6.ICMPTypeTimeExceeded

							// Extract original ICMP message from TimeExceeded body
							timeExceededBody, ok := receiveMsg.Body.(*icmp.TimeExceeded)

							// Skip IPv6 header (fixed 40 bytes)

							if !ok || len(timeExceededBody.Data) < ipv6HeaderLen+headerSizeICMP {
								log.Printf("Invalid ICMP Time-Exceeded body: %+v", receiveMsg)
								continue
							}

							// Parse the original ICMP message (protocol 58 for ICMPv6)
							originalICMPData := timeExceededBody.Data[ipv6HeaderLen:]
							originalICMPMsg, err := icmp.ParseMessage(protocolNumberICMPv6, originalICMPData)
							if err != nil {
								log.Printf("Invalid ICMP Time-Exceeded message: %+v error: %+v", receiveMsg, err)
								continue
							}

							echoBody, ok := originalICMPMsg.Body.(*icmp.Echo)
							if !ok {
								log.Printf("Invalid ICMP Time-Exceeded message: %+v", receiveMsg)
								continue
							}

							replyObject.Seq = echoBody.Seq
							replyObject.ID = echoBody.ID

						case ipv6.ICMPTypeEchoReply:
							ty = ipv6.ICMPTypeEchoReply
							icmpBody, ok := receiveMsg.Body.(*icmp.Echo)
							if !ok {
								log.Printf("failed to parse icmp body: %+v", receiveMsg)
								continue
							}
							replyObject.Seq = icmpBody.Seq
							replyObject.ID = icmpBody.ID
						default:
							log.Printf("unknown ICMP message: %+v", receiveMsg)
							continue
						}

						if replyObject.ID != icmp6tr.id {
							// silently ignore the message that is not for us
							continue
						}

						replyObject.ICMPTypeV6 = &ty
						replySubCh <- replyObject
						break
					}
				}
			}
		}()

		// launch sending goroutine
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case req, ok := <-icmp6tr.SendC:
					if !ok {
						return
					}

					wm.Body.(*icmp.Echo).Seq = req.Seq
					wm.Body.(*icmp.Echo).Data = req.Data
					wb, err := wm.Marshal(nil)
					if err != nil {
						log.Fatalf("failed to marshal icmp message: %v", err)
					}

					wcm.HopLimit = req.TTL
					dst := req.Dst
					if _, err := icmp6tr.ipv6PacketConn.WriteTo(wb, &wcm, &dst); err != nil {
						log.Fatalf("failed to write to connection: %v", err)
					}
				}
			}
		}()

		<-ctx.Done()
	}()

	return nil
}

func (icmp6tr *ICMP6Transceiver) GetSender() chan<- ICMPSendRequest {
	return icmp6tr.SendC
}

func (icmp6tr *ICMP6Transceiver) GetReceiver() chan<- chan ICMPReceiveReply {
	return icmp6tr.ReceiveC
}
