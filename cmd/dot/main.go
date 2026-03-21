package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	pkgdnsprobe "example.com/rbmq-demo/pkg/dnsprobe"
)

func main() {
	// Define the DoT server details
	address := "[2606:4700:4700::1001]:853"
	serverName := "one.one.one.one"

	tr := new(pkgdnsprobe.Transport)
	*tr = pkgdnsprobe.TransportTLS
	lookupParameter := pkgdnsprobe.LookupParameter{
		CorrelationID: "1",
		AddrPort:      address,
		Target:        "example.com",
		Transport:     tr,
		QueryType:     pkgdnsprobe.DNSQueryTypeAAAA,
		DoTServerName: serverName,
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := pkgdnsprobe.LookupDNS(ctx, lookupParameter, nil)
	if err != nil {
		log.Fatal(err)
	}
	json.NewEncoder(os.Stdout).Encode(res)
}
