package raw

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// with IP reply stripped, remains ICMPv4 PDU
func getIDSeqPMTUFromOriginIPPacket4(rawICMPReply []byte, baseDstPort int) (identifier *PacketIdentifier, err error) {
	identifier = new(PacketIdentifier)

	thepacket := gopacket.NewPacket(rawICMPReply, layers.LayerTypeICMPv4, gopacket.Default)
	if thepacket == nil {
		err = fmt.Errorf("failed to create/decode icmp packet")
		return identifier, err
	}

	icmpLayer := thepacket.Layer(layers.LayerTypeICMPv4)
	if icmpLayer == nil {
		err = fmt.Errorf("failed to extract icmp layer")
		return identifier, err
	}

	icmpPacket, ok := icmpLayer.(*layers.ICMPv4)
	if !ok {
		err = fmt.Errorf("failed to cast icmp layer to icmp packet")
		return identifier, err
	}

	ty := int(icmpPacket.TypeCode.Type())
	cd := int(icmpPacket.TypeCode.Code())
	identifier.ICMPType = &ty
	identifier.ICMPCode = &cd

	if icmpPacket.TypeCode.Type() == layers.ICMPv4TypeEchoReply {
		identifier.Id = int(icmpPacket.Id)
		identifier.Seq = int(icmpPacket.Seq)
		identifier.IPProto = int(layers.IPProtocolICMPv4)
		identifier.LastHop = true
		return identifier, err
	} else if icmpPacket.TypeCode.Type() == layers.ICMPv4TypeDestinationUnreachable {
		if icmpPacket.TypeCode.Code() == layers.ICMPv4CodeFragmentationNeeded && len(rawICMPReply) >= headerSizeICMP {
			pmtu := int(rawICMPReply[6])<<8 | int(rawICMPReply[7])
			identifier.PMTU = &pmtu
		}

		originPacket := gopacket.NewPacket(icmpPacket.Payload, layers.LayerTypeIPv4, gopacket.Default)
		if originPacket == nil {
			err = fmt.Errorf("failed to create/decode origin ip packet")
			return identifier, err
		}

		originIPLayer := originPacket.Layer(layers.LayerTypeIPv4)
		if originIPLayer == nil {
			err = fmt.Errorf("failed to extract origin ip layer")
			return identifier, err
		}

		originIPPacket, ok := originIPLayer.(*layers.IPv4)
		if !ok {
			err = fmt.Errorf("failed to cast origin ip layer to origin ip packet")
			return identifier, err
		}
		identifier.IPProto = int(originIPPacket.Protocol)

		if originIPPacket.Protocol == layers.IPProtocolICMPv4 {
			originICMPLayer := originPacket.Layer(layers.LayerTypeICMPv4)
			if originICMPLayer == nil {
				err = fmt.Errorf("failed to extract origin icmp layer")
				return identifier, err
			}

			originICMPPacket, ok := originICMPLayer.(*layers.ICMPv4)
			if !ok {
				err = fmt.Errorf("failed to cast origin icmp layer to origin icmp packet")
				return identifier, err
			}

			identifier.Id = int(originICMPPacket.Id)
			identifier.Seq = int(originICMPPacket.Seq)
			return identifier, err
		} else if originIPPacket.Protocol == layers.IPProtocolUDP {
			originUDPLayer := originPacket.Layer(layers.LayerTypeUDP)
			if originUDPLayer == nil {
				err = fmt.Errorf("failed to extract origin udp layer")
				return identifier, err
			}

			originUDPPacket, ok := originUDPLayer.(*layers.UDP)
			if !ok {
				err = fmt.Errorf("failed to cast origin udp layer to origin udp packet")
				return identifier, err
			}
			identifier.Id = int(originUDPPacket.SrcPort)
			identifier.Seq = int(originUDPPacket.DstPort) - baseDstPort
			identifier.LastHop = icmpPacket.TypeCode.Code() == layers.ICMPv4CodePort
			return identifier, err
		} else {
			err = fmt.Errorf("unknown origin ip protocol: %d", originIPPacket.Protocol)
			return identifier, err
		}
	} else if icmpPacket.TypeCode.Type() == layers.ICMPv4TypeTimeExceeded {
		originPacket := gopacket.NewPacket(icmpPacket.Payload, layers.LayerTypeIPv4, gopacket.Default)
		if originPacket == nil {
			err = fmt.Errorf("failed to create/decode origin ip packet")
			return identifier, err
		}

		originIPLayer := originPacket.Layer(layers.LayerTypeIPv4)
		if originIPLayer == nil {
			err = fmt.Errorf("failed to extract origin ip layer")
			return identifier, err
		}

		originIPPacket, ok := originIPLayer.(*layers.IPv4)
		if !ok {
			err = fmt.Errorf("failed to cast origin ip layer to origin ip packet")
			return identifier, err
		}
		identifier.IPProto = int(originIPPacket.Protocol)
		identifier.LastHop = false

		if originIPPacket.Protocol == layers.IPProtocolICMPv4 {
			originICMPLayer := originPacket.Layer(layers.LayerTypeICMPv4)
			if originICMPLayer == nil {
				err = fmt.Errorf("failed to extract origin icmp layer")
				return identifier, err
			}

			originICMPPacket, ok := originICMPLayer.(*layers.ICMPv4)
			if !ok {
				err = fmt.Errorf("failed to cast origin icmp layer to origin icmp packet")
				return identifier, err
			}

			identifier.Id = int(originICMPPacket.Id)
			identifier.Seq = int(originICMPPacket.Seq)
			return identifier, err
		} else if originIPPacket.Protocol == layers.IPProtocolUDP {
			originUDPLayer := originPacket.Layer(layers.LayerTypeUDP)
			if originUDPLayer == nil {
				err = fmt.Errorf("failed to extract origin udp layer")
				return identifier, err
			}

			originUDPPacket, ok := originUDPLayer.(*layers.UDP)
			if !ok {
				err = fmt.Errorf("failed to cast origin udp layer to origin udp packet")
				return identifier, err
			}
			identifier.Id = int(originUDPPacket.SrcPort)
			identifier.Seq = int(originUDPPacket.DstPort) - baseDstPort
			return identifier, err
		} else {
			err = fmt.Errorf("unknown origin ip protocol: %d", originIPPacket.Protocol)
			return identifier, err
		}
	} else {
		err = fmt.Errorf("unknown icmp type: %d", icmpPacket.TypeCode.Type())
		return identifier, err
	}
}
