package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

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

	tr := new(pkgdnsprobe.Transport)
	*tr = pkgdnsprobe.TransportHTTP3
	lookupParameter := pkgdnsprobe.LookupParameter{
		CorrelationID: "1",
		AddrPort:      urlStr,
		Target:        "2a00:1450:4001:81c::200e",
		Transport:     tr,
		QueryType:     pkgdnsprobe.DNSQueryTypePTR,
		DoTServerName: serverName,
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := pkgdnsprobe.LookupDNS(ctx, lookupParameter, nil)
	if err != nil {
	}
	json.NewEncoder(os.Stdout).Encode(res)
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
