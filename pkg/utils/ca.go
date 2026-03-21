package utils

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func NewCustomCAPool(caUrlsOrPaths []string) (*x509.CertPool, error) {
	var customCAs *x509.CertPool = nil
	if len(caUrlsOrPaths) > 0 {
		customCAs = x509.NewCertPool()
		for _, ca := range caUrlsOrPaths {
			if err := AppendCustomCA(customCAs, ca); err != nil {
				return nil, fmt.Errorf("failed to append CA file %s: %w", ca, err)
			}
		}
	}
	return customCAs, nil
}

func AppendCustomCA(caPool *x509.CertPool, urlorpath string) error {
	var pemData []byte
	if strings.HasPrefix(urlorpath, "https://") || strings.HasPrefix(urlorpath, "http://") {
		urlObj, err := url.Parse(urlorpath)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		resp, err := http.Get(urlObj.String())
		if err != nil {
			return fmt.Errorf("failed to get URL: %w", err)
		}
		defer resp.Body.Close()
		pemData, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		if ok := caPool.AppendCertsFromPEM(pemData); !ok {
			return fmt.Errorf("failed to append CA to pool")
		}
		return nil
	}
	pemData, err := os.ReadFile(urlorpath)
	if err != nil {
		return fmt.Errorf("failed to open %s as a file: %w", urlorpath, err)
	}
	if ok := caPool.AppendCertsFromPEM(pemData); !ok {
		return fmt.Errorf("failed to append CA to pool")
	}
	return nil
}

func GetExtendedCAPool(additionalCAPaths []string) (*x509.CertPool, error) {
	caPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.New("can't obtain system's CA pool")
	}
	caPool = caPool.Clone()
	for _, caPath := range additionalCAPaths {
		caData, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file %s: %w", caPath, err)
		}
		if !caPool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", caPath)
		}
	}
	return caPool, nil
}
