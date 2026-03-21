package main

import (
	"context"
	"io"
	"log"
	"net/http"

	dns "codeberg.org/miekg/dns"
)

func main() {
	urlStr := "https://dns.google/dns-query"
	client := http.DefaultClient
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

	req.Header.Set("Accept", mime)
	req.Header.Set("Content-Type", mime)

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

	log.Printf("Got %d bytes response", len(respContent))

	ansM := new(dns.Msg)
	ansM.Data = respContent
	if err := ansM.Unpack(); err != nil {
		log.Fatal(err)
	}

	for idx, ans := range ansM.Answer {
		log.Printf("[%d] ans: %s", idx, ans.String())
	}
}
