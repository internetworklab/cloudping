package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"time"
)

type Transport string

const (
	TransportUDP Transport = "udp"
	TransportTCP Transport = "tcp"
)

type DNSQueryType string

const (
	DNSQueryTypeA     DNSQueryType = "a"
	DNSQueryTypeAAAA  DNSQueryType = "aaaa"
	DNSQueryTypeCNAME DNSQueryType = "cname"
)

type LookupParameter struct {
	AddrPort  netip.AddrPort
	Target    string
	Timeout   time.Duration
	Transport Transport
	QueryType DNSQueryType
}

type QueryResult struct {
	Server           netip.AddrPort
	Target           string
	QueryType        DNSQueryType
	Answers          []interface{}
	Error            error
	IOTimeout        bool
	NoSuchHost       bool
	Elapsed          time.Duration
	StartedAt        time.Time
	TimeoutSpecified time.Duration
}

func (qr *QueryResult) Log() {
	log.Printf("%d answers for type %s: target: %s, elapsed: %v, error: %v, timeout: %v, no such host: %v", len(qr.Answers), qr.QueryType, qr.Target, qr.Elapsed.String(), qr.Error != nil, qr.IOTimeout, qr.NoSuchHost)
	for _, ans := range qr.Answers {
		ansStr, err := wrappedAnsToString(ans, qr.QueryType)
		if err != nil {
			log.Fatalf("failed to convert answer to string: %v", err)
		}
		log.Printf("answer: %s", ansStr)
	}
}

func analyzeError(err error, queryResult *QueryResult) bool {
	queryResult.Error = err
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		queryResult.IOTimeout = true
		return true
	} else if errors.Is(err, context.Canceled) {
		queryResult.IOTimeout = true
		return true
	} else if _, ok := err.(*net.DNSError); ok {
		queryResult.NoSuchHost = true
		return true
	} else {
		return false
	}
}

// returns: answers, error
func lookupDNS(parameter LookupParameter) (*QueryResult, error) {

	transport := parameter.Transport
	addrport := parameter.AddrPort
	target := parameter.Target
	timeout := parameter.Timeout
	queryType := parameter.QueryType
	queryResult := new(QueryResult)
	queryResult.Target = target
	queryResult.QueryType = queryType
	queryResult.Answers = make([]interface{}, 0)
	queryResult.Server = addrport
	queryResult.TimeoutSpecified = timeout
	queryResult.StartedAt = time.Now()
	defer func() {
		queryResult.Elapsed = time.Since(queryResult.StartedAt)
	}()

	resolver := net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			if transport == TransportUDP {
				udpaddr := net.UDPAddrFromAddrPort(addrport)
				if udpaddr == nil {
					return nil, fmt.Errorf("failed to get udpaddr from %s", addrport.String())
				}
				return net.DialUDP("udp", nil, udpaddr)
			} else if transport == TransportTCP {
				tcpaddr := net.TCPAddrFromAddrPort(addrport)
				if tcpaddr == nil {
					return nil, fmt.Errorf("failed to get tcpaddr from %s", addrport.String())
				}
				return net.DialTCP("tcp", nil, tcpaddr)
			} else {
				return nil, fmt.Errorf("transport is not specified or invalid transport: %s", transport)
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch queryType {
	case DNSQueryTypeA, DNSQueryTypeAAAA:
		ipPref := "ip"
		if queryType == DNSQueryTypeA {
			ipPref = "ip4"
		} else if queryType == DNSQueryTypeAAAA {
			ipPref = "ip6"
		}
		answers, err := resolver.LookupIP(ctx, ipPref, target)
		if err != nil && !analyzeError(err, queryResult) {
			return nil, fmt.Errorf("failed to lookup ip (type %s) for %s: %v", queryType, target, err)
		}

		for _, ans := range answers {
			queryResult.Answers = append(queryResult.Answers, ans)
		}
		return queryResult, nil
	case DNSQueryTypeCNAME:
		answer, err := resolver.LookupCNAME(ctx, target)
		if err != nil && !analyzeError(err, queryResult) {
			return nil, fmt.Errorf("failed to lookup ip (type %s) for %s: %v", queryType, target, err)
		}

		if answer != "" {
			queryResult.Answers = append(queryResult.Answers, answer)
		}
		return queryResult, nil
	default:
		return nil, fmt.Errorf("invalid query type: %s", queryType)
	}
}

func wrappedAnsToString(ans interface{}, qtype DNSQueryType) (string, error) {
	switch qtype {
	case DNSQueryTypeA:
		ip, ok := ans.(net.IP)
		if !ok {
			return "", fmt.Errorf("answer is not a net.IP: %v", ans)
		}
		return ip.String(), nil
	case DNSQueryTypeAAAA:
		ip, ok := ans.(net.IP)
		if !ok {
			return "", fmt.Errorf("answer is not a net.IP: %v", ans)
		}
		return ip.String(), nil
	case DNSQueryTypeCNAME:
		cname, ok := ans.(string)
		if !ok {
			return "", fmt.Errorf("answer is not a string: %v", ans)
		}
		return cname, nil
	default:
		return "", fmt.Errorf("unknown query type: %s", qtype)
	}
}

func main() {
	addrport, err := netip.ParseAddrPort("8.8.4.4:53")
	if err != nil {
		log.Fatalf("failed to parse addrport: %v", err)
	}

	targets := []string{
		"www.google.com",
		"www.baidu.com",
		"www.12023bingo2398sjdsfjoidsjo.com",
	}
	queryTypes := []DNSQueryType{
		DNSQueryTypeA,
		DNSQueryTypeAAAA,
		DNSQueryTypeCNAME,
	}

	for _, target := range targets {
		for _, queryType := range queryTypes {
			parameter := LookupParameter{
				AddrPort:  addrport,
				Target:    target,
				Timeout:   3000 * time.Millisecond,
				Transport: TransportUDP,
				QueryType: queryType,
			}
			log.Printf("looking up type %s dns for target %s", queryType, target)
			queryResult, err := lookupDNS(parameter)
			if err != nil {
				log.Printf("failed to lookup type %s dns for target %s: %v", queryType, target, err)
			}
			queryResult.Log()
		}
	}

}
