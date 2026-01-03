package main

import (
	"encoding/base64"
	"log"
	"os"

	pkgraw "example.com/rbmq-demo/pkg/raw"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

func main() {
	path := "cmd/packettoobit6/data.txt"
	base64txt, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read base64txt: %v", err)
	}

	rb, err := base64.StdEncoding.DecodeString(string(base64txt))
	if err != nil {
		log.Fatalf("failed to decode base64txt: %v", err)
	}

	log.Printf("len: %d\n", len(rb))

	receiveMsg, err := icmp.ParseMessage(int(layers.IPProtocolICMPv6), rb)
	log.Printf("icmp type: %v, code: %v", receiveMsg.Type, receiveMsg.Code)
	if receiveMsg.Type != ipv6.ICMPTypePacketTooBig {
		panic("not packet too big")
	}

	// usually occurs when the user is intentionally performing a PMTU trace
	packetTooBigMsg, ok := receiveMsg.Body.(*icmp.PacketTooBig)
	if !ok {
		log.Fatalf("failed to cast packet too big body to *icmp.PacketTooBig")
	}

	originLen := len(packetTooBigMsg.Data)
	log.Printf("origin len: %d", originLen)

	portBase := 33433
	originPktIdentifier, err := pkgraw.ExtractPacketInfoFromOriginIP6(packetTooBigMsg.Data, portBase)
	if err != nil {
		log.Fatalf("failed to extract packet info from origin ip6 packet: %v", err)
	}
	log.Printf("origin pkt identifier: %s", originPktIdentifier.String())
}
