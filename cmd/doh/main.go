package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"codeberg.org/miekg/dns"
	pkgdnsprobe "example.com/rbmq-demo/pkg/dnsprobe"
	pkgutils "example.com/rbmq-demo/pkg/utils"
	quicHTTP3 "github.com/quic-go/quic-go/http3"
)

const serverName string = "dns.google"
const urlStr string = "https://[2001:4860:4860::8888]/dns-query"

func getDemoRequest() *http.Request {

	ctx := context.Background()
	mime := "application/dns-message"

	m := dns.NewMsg("miek.nl", dns.TypeMX)
	m.ID = dns.ID()
	m.RecursionDesired = true
	if err := m.Pack(); err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, m)
	if err != nil {
		log.Fatal(err)
	}

	req.Host = serverName

	req.Header.Set("Accept", mime)
	req.Header.Set("Content-Type", mime)
	return req
}

func dealDNSResp(respBody []byte) {
	log.Printf("Got %d bytes response", len(respBody))

	ansM := new(dns.Msg)
	ansM.Data = respBody
	if err := ansM.Unpack(); err != nil {
		log.Fatal(err)
	}

	for idx, ans := range ansM.Answer {
		log.Printf("[%d] ans: %s, data: %s", idx, ans.String(), ans.Data().String())
	}
}

func dohDemo() {
	client := http.DefaultClient
	req := getDemoRequest()

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Response proto: %s", resp.Proto)
	log.Printf("Response Code: %s", resp.Status)

	defer resp.Body.Close()
	respContent, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	dealDNSResp(respContent)
}

func doh3Demo(addCA []string) error {

	tr := &quicHTTP3.Transport{}
	if len(addCA) > 0 {
		caPool, err := pkgutils.GetExtendedCAPool(addCA)
		if err != nil {
			return err
		}
		tr.TLSClientConfig = &tls.Config{
			RootCAs:    caPool,
			ServerName: serverName,
		}
	}

	req := getDemoRequest()

	resp, err := tr.RoundTrip(req)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Response proto: %s", resp.Proto)
	log.Printf("Response Code: %s", resp.Status)

	defer resp.Body.Close()
	respContent, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	dealDNSResp(respContent)

	return nil
}

func dnsProbeDemo() {

	ctx := context.Background()

	transportsToTry := []pkgdnsprobe.Transport{
		pkgdnsprobe.TransportUDP,
		pkgdnsprobe.TransportTCP,
		pkgdnsprobe.TransportTLS,
		pkgdnsprobe.TransportHTTP2,
		pkgdnsprobe.TransportHTTP3,
	}
	dnsServerMap := make(map[pkgdnsprobe.Transport]string)
	dnsServerMap[pkgdnsprobe.TransportUDP] = "8.8.8.8"
	dnsServerMap[pkgdnsprobe.TransportTCP] = "8.8.8.8"
	dnsServerMap[pkgdnsprobe.TransportTLS] = "8.8.8.8"
	dnsServerMap[pkgdnsprobe.TransportHTTP2] = urlStr
	dnsServerMap[pkgdnsprobe.TransportHTTP3] = urlStr
	for _, tr := range transportsToTry {
		lookupParameter := pkgdnsprobe.LookupParameter{
			CorrelationID: "1",
			AddrPort:      dnsServerMap[tr],
			Target:        "142.251.127.139",
			QueryType:     pkgdnsprobe.DNSQueryTypePTR,
			DoTServerName: serverName,
		}
		lookupParameter.Transport = &tr

		log.Printf("Trying transport %s, server %s", tr, lookupParameter.AddrPort)
		res, err := pkgdnsprobe.LookupDNS(ctx, lookupParameter, nil)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(res)
	}
}

func main() {
	// log.Printf("Starting DoH (DNS over HTTP/2) demo")
	// dohDemo()

	// log.Printf("Starting DoH (DNS over HTTP/3) demo")
	// if err := doh3Demo([]string{}); err != nil {
	// 	log.Fatal(err)
	// }

	log.Printf("Starting DNS Probe demo")
	dnsProbeDemo()
}
