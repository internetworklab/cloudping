package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func NewMRTQueryProxyHandler(backendURL string, stripPrefix string) (http.Handler, error) {
	backend, err := url.Parse(backendURL)
	if err != nil {
		return nil, fmt.Errorf("invalid MRT query service backend URL %q: %w", backendURL, err)
	}

	revProxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(backend)
			pr.SetXForwarded()
			// Strip the prefix so e.g. /proxy/mrt/providers -> /providers
			pr.Out.URL.Path = strings.TrimPrefix(pr.In.URL.Path, stripPrefix)
			if pr.Out.URL.Path == "" {
				pr.Out.URL.Path = "/"
			}
		},
	}

	return revProxy, nil
}
