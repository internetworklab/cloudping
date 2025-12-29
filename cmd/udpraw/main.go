package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func main() {
	pktRaw := []byte{0x9e, 0x54, 0x3, 0x8e, 0x71, 0x8f, 0xe4, 0x5f, 0x1, 0x69, 0x1e, 0xb, 0x8, 0x0, 0x45, 0xc0, 0x0, 0x44, 0xd6, 0x5d, 0x0, 0x0, 0x40, 0x1, 0x1a, 0x33, 0xc0, 0xa8, 0x4, 0x1, 0xc0, 0xa8, 0x4, 0x17, 0xb, 0x0, 0xc5, 0xf0, 0x0, 0x0, 0x0, 0x0, 0x45, 0x0, 0x0, 0x28, 0xe0, 0xfb, 0x0, 0x0, 0x1, 0x11, 0x7, 0xff, 0xc0, 0xa8, 0x4, 0x17, 0x8, 0x8, 0x4, 0x4, 0xe0, 0xfa, 0x82, 0x9b, 0x0, 0x14, 0xcb, 0x64, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}

	log.Printf("pktRaw, length: %d", len(pktRaw))

	packet := gopacket.NewPacket(pktRaw, layers.LayerTypeEthernet, gopacket.Default)
	if packet == nil {
		log.Fatalf("failed to create/decode packet")
	}

	for _, layer := range packet.Layers() {
		log.Printf("Found layer: %s", layer.LayerType().String())
	}

	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		log.Fatalf("failed to find ipv4 layer")
	}
	ipPacket, ok := ipLayer.(*layers.IPv4)
	if !ok {
		log.Fatalf("failed to cast ipv4 layer to ipv4 packet")
	}
	log.Printf("IPv4, SRC: %s, DST: %s, TTL: %d, Protocol: %d", ipPacket.SrcIP.String(), ipPacket.DstIP.String(), ipPacket.TTL, ipPacket.Protocol)

	icmpLayer := packet.Layer(layers.LayerTypeICMPv4)
	if icmpLayer == nil {
		log.Fatalf("failed to find icmpv4 layer")
	}
	icmpPacket, ok := icmpLayer.(*layers.ICMPv4)
	if !ok {
		log.Fatalf("failed to cast icmpv4 layer to icmpv4 packet")
	}
	log.Printf("ICMPv4, ID: %d, Seq: %d, Type: %d, Code: %d", icmpPacket.Id, icmpPacket.Seq, icmpPacket.TypeCode.Type(), icmpPacket.TypeCode.Code())

	originPacket := gopacket.NewPacket(icmpPacket.Payload, layers.LayerTypeIPv4, gopacket.Default)
	if originPacket == nil {
		log.Fatalf("failed to create/decode origin ip packet")
	}

	log.Printf("Decoding origin ip packet ...")
	for _, layer := range originPacket.Layers() {
		log.Printf("Found layer: %s", layer.LayerType().String())
	}

	originIPLayer := originPacket.Layer(layers.LayerTypeIPv4)
	if originIPLayer == nil {
		log.Fatalf("failed to find origin ip layer")
	}
	originIPPacket, ok := originIPLayer.(*layers.IPv4)
	if !ok {
		log.Fatalf("failed to cast origin ip layer to origin ip packet")
	}

	log.Printf("Origin IP, SRC: %s, DST: %s, TTL: %d, Protocol: %d", originIPPacket.SrcIP.String(), originIPPacket.DstIP.String(), originIPPacket.TTL, originIPPacket.Protocol)

	udpLayer := originPacket.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		log.Fatalf("failed to find udp layer")
	}
	udpPacket, ok := udpLayer.(*layers.UDP)
	if !ok {
		log.Fatalf("failed to cast udp layer to udp packet")
	}
	log.Printf("UDP, SrcPort: %d, DstPort: %d, Length: %d", udpPacket.SrcPort, udpPacket.DstPort, udpPacket.Length)

	udpPayloadContent := make([]string, 0)
	for _, byte := range udpPacket.Payload {
		udpPayloadContent = append(udpPayloadContent, fmt.Sprintf("%02x", byte))
	}
	log.Printf("UDP payload size: %d, content: %s", len(udpPayloadContent), strings.Join(udpPayloadContent, " "))
}
