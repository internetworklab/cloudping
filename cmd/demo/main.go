package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	pkghttpprobe "example.com/rbmq-demo/pkg/httpprobe"
	pkgpinger "example.com/rbmq-demo/pkg/pinger"
	"github.com/google/uuid"
)

func main() {
	url := "https://www.google.com/robots.txt"

	maxHeadersFields := 100
	responseSizeLimit := int64(4 * 1024)
	probe := pkghttpprobe.HTTPProbe{
		URL:                   url,
		Proto:                 pkghttpprobe.HTTPProtoHTTP3,
		NumHeadersFieldsLimit: &maxHeadersFields,
		SizeLimit:             &responseSizeLimit,
		CorrelationID:         uuid.New().String(),
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	pinger := pkgpinger.HTTPPinger{
		Requests: []pkghttpprobe.HTTPProbe{probe},
	}

	encoder := json.NewEncoder(os.Stdout)
	for ev := range pinger.Ping(ctx) {
		encoder.Encode(ev)
	}
}
