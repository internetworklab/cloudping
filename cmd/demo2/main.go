package main

import (
	"fmt"
	"net/url"
	"strings"
)

func main() {
	urlsList := []string{
		"https://example.com",
		"https://example.com/",
		"https://example.com/1.2.3.4",
		"https://example.com/bc/1.2.3.4",
		"https://example.com/api/proxy/ipregistry/1.2.3.4",
		"https://example.com/1.2.3.4/cd?key=bb",
		"https://example.com/1.2.3.4/aaa?key=",
		"https://example.com/1.2.3.4/def",
	}
	for i, u := range urlsList {
		uObj, _ := url.Parse(u)
		ip := uObj.Path
		if x, ok := strings.CutPrefix(ip, "/"); ok {
			ip = x
		}
		ipSegs := strings.FieldsFunc(ip, func(r rune) bool { return r == '/' })
		if len(ipSegs) > 0 {
			ip = ipSegs[len(ipSegs)-1]
		}
		fmt.Printf("%s -> Path: %s, ip: %s, ipSegs: %s\n", urlsList[i], uObj.Path, ip, strings.Join(ipSegs, ", "))
	}
}
