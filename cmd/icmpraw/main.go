package main

import (
	"log"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func main() {
	pktRaw := []byte{0x9e, 0x54, 0x3, 0x8e, 0x71, 0x8f, 0xe4, 0x5f, 0x1, 0x69, 0x1e, 0xb, 0x8, 0x0, 0x45, 0xc0, 0x0, 0x5c, 0xd2, 0x4c, 0x0, 0x0, 0x40, 0x1, 0x1e, 0x2c, 0xc0, 0xa8, 0x4, 0x1, 0xc0, 0xa8, 0x4, 0x17, 0xb, 0x0, 0xf4, 0xff, 0x0, 0x0, 0x0, 0x0, 0x45, 0x0, 0x0, 0x40, 0x68, 0xb2, 0x0, 0x0, 0x1, 0x1, 0x80, 0x40, 0xc0, 0xa8, 0x4, 0x17, 0x8, 0x8, 0x4, 0x4, 0x8, 0x0, 0x16, 0x53, 0x60, 0xc4, 0x80, 0xe8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}

	log.Printf("pkgRaw, length: %d", len(pktRaw))

	packet := gopacket.NewPacket(pktRaw, layers.LayerTypeEthernet, gopacket.Default)
	if packet == nil {
		log.Fatalf("failed to create/decode packet")
	}

	for _, layer := range packet.Layers() {
		log.Printf("Found layer: %s", layer.LayerType().String())
	}

	ethernetLayer := packet.Layer(layers.LayerTypeEthernet)
	if ethernetLayer == nil {
		log.Fatalf("failed to find ethernet layer")
	}

	ethernetPacket, ok := ethernetLayer.(*layers.Ethernet)
	if !ok {
		log.Fatalf("failed to cast ethernet layer to ethernet packet")
	}
	log.Printf("Ethernet, SRC: %s, DST: %s", ethernetPacket.SrcMAC.String(), ethernetPacket.DstMAC.String())

	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		log.Fatalf("failed to find ipv4 layer")
	}
	ipPacket, ok := ipLayer.(*layers.IPv4)
	if !ok {
		log.Fatalf("failed to cast ipv4 layer to ipv4 packet")
	}
	log.Printf("IPv4, SRC: %s, DST: %s, TTL: %d", ipPacket.SrcIP.String(), ipPacket.DstIP.String(), ipPacket.TTL)

	icmpLayer := packet.Layer(layers.LayerTypeICMPv4)
	if icmpLayer == nil {
		log.Fatalf("failed to find icmpv4 layer")
	}
	icmpPacket, ok := icmpLayer.(*layers.ICMPv4)
	if !ok {
		log.Fatalf("failed to cast icmpv4 layer to icmpv4 packet")
	}
	log.Printf("ICMPv4, ID: %d, Seq: %d, Type: %d, Code: %d", icmpPacket.Id, icmpPacket.Seq, icmpPacket.TypeCode.Type(), icmpPacket.TypeCode.Code())

	originIPPacket := gopacket.NewPacket(icmpPacket.Payload, layers.LayerTypeIPv4, gopacket.Default)
	if originIPPacket == nil {
		log.Fatalf("failed to create/decode origin ip packet")
	}

	log.Printf("Decoding origin ip packet ...")
	for _, layer := range originIPPacket.Layers() {
		log.Printf("Found layer: %s", layer.LayerType().String())
	}

	sentIPLayer := originIPPacket.Layer(layers.LayerTypeIPv4)
	if sentIPLayer == nil {
		log.Fatalf("failed to find sent ip layer")
	}
	sentIPPacket, ok := sentIPLayer.(*layers.IPv4)
	if !ok {
		log.Fatalf("failed to cast sent ip layer to sent ip packet")
	}
	log.Printf("Sent IP, SRC: %s, DST: %s, TTL: %d", sentIPPacket.SrcIP.String(), sentIPPacket.DstIP.String(), sentIPPacket.TTL)

	sentICMPLayer := originIPPacket.Layer(layers.LayerTypeICMPv4)
	if sentICMPLayer == nil {
		log.Fatalf("failed to find sent icmpv4 layer")
	}
	sentICMPPacket, ok := sentICMPLayer.(*layers.ICMPv4)
	if !ok {
		log.Fatalf("failed to cast sent icmpv4 layer to sent icmpv4 packet")
	}
	log.Printf("Sent ICMP, ID: %d, Seq: %d, Type: %d, Code: %d", sentICMPPacket.Id, sentICMPPacket.Seq, sentICMPPacket.TypeCode.Type(), sentICMPPacket.TypeCode.Code())
}
