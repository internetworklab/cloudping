package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkgutils "example.com/rbmq-demo/pkg/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/ipv4"
)

type PacketInfo struct {
	Hdr     *ipv4.Header
	Payload []byte
	CtrlMsg *ipv4.ControlMessage
	TCP     *layers.TCP
}

func getPackets(rawConn *ipv4.RawConn) <-chan PacketInfo {
	rbCh := make(chan PacketInfo)
	rb := make([]byte, pkgutils.GetMaximumMTU())

	go func() {
		defer close(rbCh)

		for {
			hdr, payload, ctrlMsg, err := rawConn.ReadFrom(rb)
			if err != nil {
				log.Printf("failed to read from raw connection: %v", err)
				return
			}
			pktInfo := PacketInfo{}
			pktInfo.Hdr = hdr
			pktInfo.Payload = make([]byte, hdr.TotalLen)
			copy(pktInfo.Payload, payload)
			pktInfo.CtrlMsg = ctrlMsg
			rbCh <- pktInfo
		}

	}()
	return rbCh
}

func filterPackets(rbCh <-chan PacketInfo, localPort int) <-chan PacketInfo {
	filteredCh := make(chan PacketInfo)
	go func() {
		defer close(filteredCh)
		for pktInfo := range rbCh {
			hdr := pktInfo.Hdr
			if hdr == nil {
				continue
			}
			if hdr.Protocol != int(layers.IPProtocolTCP) {
				continue
			}

			packet := gopacket.NewPacket(pktInfo.Payload, layers.LayerTypeTCP, gopacket.Default)
			if packet == nil {
				continue
			}

			tcpLayer := packet.Layer(layers.LayerTypeTCP)
			if tcpLayer == nil {
				continue
			}

			tcp, ok := tcpLayer.(*layers.TCP)
			if !ok {
				continue
			}

			if int(tcp.DstPort) != localPort {
				continue
			}
			newPacket := new(PacketInfo)
			*newPacket = pktInfo
			newPacket.TCP = tcp
			filteredCh <- *newPacket
		}
	}()

	return filteredCh
}

func main() {

	dstIP := net.ParseIP("172.17.0.7")

	handle, err := netlink.NewHandle()
	if err != nil {
		log.Fatalf("failed to create netlink handle: %v", err)
	}
	defer handle.Close()

	routes, err := handle.RouteGet(dstIP)
	if err != nil {
		log.Fatalf("failed to get routes for %s: %v", dstIP.String(), err)
	}

	if len(routes) == 0 {
		log.Fatalf("no routes found for %s", dstIP.String())
	}

	route := routes[0]
	srcIP := route.Src

	ctx := context.Background()
	listenConfig := net.ListenConfig{}

	tcpListener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Fatalf("failed to listen on tcp: %v", err)
	}
	localPort := tcpListener.Addr().(*net.TCPAddr).Port
	defer tcpListener.Close()
	log.Printf("listening on %s", tcpListener.Addr().String())

	dstPort := 80
	log.Printf("dst: %s:%d, src: %s:%d", dstIP.String(), dstPort, srcIP.String(), localPort)

	ipProtoTCP := fmt.Sprintf("%d", int(layers.IPProtocolTCP))

	ln, err := listenConfig.ListenPacket(ctx, "ip4:"+ipProtoTCP, "0.0.0.0")
	if err != nil {
		log.Fatalf("failed to create raw tcp/ip socket: %v", err)
	}

	defer ln.Close()

	log.Printf("listening on %s", ln.LocalAddr().String())

	rawConn, err := ipv4.NewRawConn(ln)
	if err != nil {
		log.Fatalf("failed to create raw connection: %v", err)
	}

	log.Printf("raw connection created")

	rbCh := getPackets(rawConn)
	filteredCh := filterPackets(rbCh, localPort)

	go func() {
		<-time.After(3 * time.Second)
		ttl := 63
		ipProto := layers.IPProtocolTCP
		var flags layers.IPv4Flag
		flags = flags | layers.IPv4DontFragment
		hdrLayer := &layers.IPv4{
			SrcIP:    srcIP,
			DstIP:    dstIP,
			TTL:      uint8(ttl),
			Protocol: ipProto,
			Flags:    flags,
		}

		// length of tcp header, in unit of words (4 bytes)
		// so, 5 words means 5 word * 4 bytes/word = 20 bytes
		tcpHdrLenNWords := 5
		tcpLayer := &layers.TCP{
			SrcPort:    layers.TCPPort(localPort),
			DstPort:    layers.TCPPort(dstPort),
			Seq:        1000,
			Ack:        0,
			SYN:        true,
			DataOffset: uint8(tcpHdrLenNWords),
		}
		tcpLayer.SetNetworkLayerForChecksum(hdrLayer)
		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
		}
		err := gopacket.SerializeLayers(buf, opts, tcpLayer)
		if err != nil {
			log.Fatalf("failed to serialize tcp layer: %v", err)
		}
		wb := buf.Bytes()
		hdr := &ipv4.Header{
			Version:  ipv4.Version,
			Len:      ipv4.HeaderLen,
			TotalLen: ipv4.HeaderLen + len(wb),
			TTL:      ttl,
			Protocol: int(ipProto),
			Dst:      dstIP,
			Flags:    ipv4.HeaderFlags(flags),
		}
		err = rawConn.WriteTo(hdr, wb, nil)
		if err != nil {
			log.Fatalf("failed to write to raw connection: %v", err)
		}
		log.Printf("sent tcp syn to %s:%d", dstIP.String(), dstPort)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case sig := <-sigs:
			log.Printf("received signal: %s", sig.String())
			return
		case pktInfo, ok := <-filteredCh:
			if !ok {
				log.Printf("filteredCh is closed")
			}
			tcp := pktInfo.TCP
			if tcp == nil {
				continue
			}
			hdr := pktInfo.Hdr
			if hdr == nil {
				continue
			}
			log.Printf("Received TCP, SRC: %s:%d, DST: %s:%d, Seq: %d, SYN: %v, ACK: %v", hdr.Src.String(), tcp.SrcPort, hdr.Dst.String(), tcp.DstPort, tcp.Seq, tcp.SYN, tcp.ACK)
		}
	}
}
