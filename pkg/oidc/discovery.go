package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DiscoveryDocument represents the OIDC Discovery document
// as specified by OpenID Connect Discovery 1.0.
// See: https://openid.net/specs/openid-connect-discovery-1_0.html
type DiscoveryDocument struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserInfoEndpoint      string   `json:"userinfo_endpoint,omitempty"`
	RevocationEndpoint    string   `json:"revocation_endpoint,omitempty"`
	JWKSEndpoint          string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported,omitempty"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported,omitempty"`
}

// DiscoveryCache fetches and caches the OIDC discovery document with a TTL.
type DiscoveryCache struct {
	mu        sync.RWMutex
	document  *DiscoveryDocument
	fetchedAt time.Time
	ttl       time.Duration
	issuerURL string
}

// NewDiscoveryCache creates a new DiscoveryCache for the given issuer URL.
// If ttl <= 0, a default of 1 hour is used.
func NewDiscoveryCache(issuerURL string, ttl time.Duration) *DiscoveryCache {
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}
	return &DiscoveryCache{
		issuerURL: issuerURL,
		ttl:       ttl,
	}
}

// Get returns the cached discovery document, fetching it if expired or not yet loaded.
func (c *DiscoveryCache) Get(ctx context.Context) (*DiscoveryDocument, error) {
	c.mu.RLock()
	if c.document != nil && time.Since(c.fetchedAt) < c.ttl {
		doc := c.document
		c.mu.RUnlock()
		return doc, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.document != nil && time.Since(c.fetchedAt) < c.ttl {
		return c.document, nil
	}

	doc, err := fetchDiscoveryDocument(ctx, c.issuerURL)
	if err != nil {
		// Return stale document if available, rather than failing completely
		if c.document != nil {
			return c.document, nil
		}
		return nil, err
	}

	c.document = doc
	c.fetchedAt = time.Now()
	return doc, nil
}

// fetchDiscoveryDocument fetches the OIDC discovery document from the issuer's well-known endpoint.
func fetchDiscoveryDocument(ctx context.Context, issuerURL string) (*DiscoveryDocument, error) {
	discoveryURL := strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch discovery document from %s: %w", discoveryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery document request to %s returned status %d", discoveryURL, resp.StatusCode)
	}

	doc := new(DiscoveryDocument)
	if err := json.NewDecoder(resp.Body).Decode(doc); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}

	if doc.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("discovery document missing required field: authorization_endpoint")
	}
	if doc.TokenEndpoint == "" {
		return nil, fmt.Errorf("discovery document missing required field: token_endpoint")
	}

	return doc, nil
}
