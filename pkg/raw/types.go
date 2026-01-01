package raw

import (
	"net"
	"time"

	pkgipinfo "example.com/rbmq-demo/pkg/ipinfo"
)

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

	PeerRawIP *net.IPAddr `json:"-"`

	LastHop bool

	PeerRDNS []string

	ReceivedAt time.Time

	// ICMPv4 and ICMPv6 has different semantics for Type and Code,
	// so a dedicated field for indicating IP version is needed.
	INetFamily int
	ICMPType   *int
	ICMPCode   *int

	// IPProtocol of the ip packet that was sent, not reply
	// when some node reply with an icmp error message, we can extract the origin ip packet
	// out from the icmp payload.
	IPProto int

	SetMTUTo            *int
	ShrinkICMPPayloadTo *int `json:"-"`

	// below are left for ip information provider
	PeerASN           *string
	PeerLocation      *string
	PeerISP           *string
	PeerExactLocation *pkgipinfo.ExactLocation
}

type GeneralICMPTransceiver interface {
	GetSender() <-chan chan ICMPSendRequest
	GetReceiver() <-chan ICMPReceiveReply
	Close() error
}

const ipv4HeaderLen int = 20
const ipv6HeaderLen int = 40
const udpHeaderLen int = 8
const headerSizeICMP int = 8
const protocolNumberICMPv4 int = 1
const protocolNumberICMPv6 int = 58
const icmpCodeFragmentationNeeded int = 4
