package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// todo: rewrite dns by https://codeberg.org/miekg/dns

type DoHNetConnWrapper struct {
	urlObj      *url.URL
	httpClient  *http.Client
	ctx         context.Context
	closed      atomic.Bool
	queryBuffer bytes.Buffer
	response    []byte
	respErr     error
	once        sync.Once
	mu          sync.Mutex
}

func NewDoHNetConnWrapper(ctx context.Context, urlStr string, httpClient *http.Client) (*DoHNetConnWrapper, error) {
	log.Printf("[dbg] called, %s", urlStr)

	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	conn := &DoHNetConnWrapper{
		urlObj:     urlObj,
		httpClient: httpClient,
		ctx:        ctx,
	}

	return conn, nil
}

func (conn *DoHNetConnWrapper) sendRequest() {
	conn.once.Do(func() {
		req, err := http.NewRequestWithContext(conn.ctx, http.MethodPost, conn.urlObj.String(), bytes.NewReader(conn.queryBuffer.Bytes()))
		if err != nil {
			conn.respErr = err
			return
		}
		req.Header.Set("Content-Type", "application/dns-message")
		req.Header.Set("Accept", "application/dns-message")

		resp, err := conn.httpClient.Do(req)
		if err != nil {
			conn.respErr = err
			return
		}
		defer resp.Body.Close()

		log.Printf("[dbg] status: %s", resp.Status)

		conn.mu.Lock()
		defer conn.mu.Unlock()
		conn.response, conn.respErr = io.ReadAll(resp.Body)
	})
}

func (conn *DoHNetConnWrapper) Read(b []byte) (int, error) {
	conn.sendRequest()

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.respErr != nil {
		return 0, conn.respErr
	}
	if len(conn.response) == 0 {
		return 0, io.EOF
	}
	n := copy(b, conn.response)
	conn.response = conn.response[n:]
	return n, nil
}

func (conn *DoHNetConnWrapper) Write(b []byte) (int, error) {
	return conn.queryBuffer.Write(b)
}

func (conn *DoHNetConnWrapper) doClose() error {
	return nil
}

func (conn *DoHNetConnWrapper) Close() error {
	for {
		closed := conn.closed.Load()
		if closed {
			// closed by other goroutine
			return nil
		}
		if changed := conn.closed.CompareAndSwap(closed, true); changed {
			return conn.doClose()
		}
	}
}

func (conn *DoHNetConnWrapper) LocalAddr() net.Addr {
	return nil
}

func (conn *DoHNetConnWrapper) RemoteAddr() net.Addr {
	return nil
}

func (conn *DoHNetConnWrapper) SetReadDeadline(t time.Time) error {
	// log.Printf("[dbg] set read deadline: %s", t.String())
	// todo
	// Note: A read deadline will not try to close anything, it just reminds the reader currently there's no data and come back later
	return nil
}

func (conn *DoHNetConnWrapper) SetWriteDeadline(t time.Time) error {
	// log.Printf("[dbg] set write deadline: %s", t.String())
	// todo
	// Note: A write deadline will not ever try to close anything either.
	return nil
}

func (conn *DoHNetConnWrapper) SetDeadline(t time.Time) error {
	if err := conn.SetReadDeadline(t); err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func main() {
	urlStr := "https://cloudflare-dns.com/dns-query"
	client := http.DefaultClient
	ctx := context.Background()

	resolver := net.Resolver{
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return NewDoHNetConnWrapper(ctx, urlStr, client)
		},
		PreferGo: true,
	}

	addrs, err := resolver.LookupIP(ctx, "ip6", "www.cloudflare.com")
	if err != nil {
		log.Fatal(err)
	}

	for _, a := range addrs {
		log.Println("Found:", a.String())
	}
}
