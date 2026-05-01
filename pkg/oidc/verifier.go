package oidc

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
)

// ProviderCache lazily initialises and caches an *oidc.Provider for a given
// issuer URL. The provider is thread-safe once created and reuses the same
// underlying HTTP transport.
type ProviderCache struct {
	mu       sync.Once
	provider *oidc.Provider
	issuer   string
	err      error
}

// NewProviderCache creates a new ProviderCache for the given issuer URL.
func NewProviderCache(issuer string) *ProviderCache {
	return &ProviderCache{issuer: issuer}
}

// Get returns the cached *oidc.Provider, creating it on first call.
func (c *ProviderCache) Get(ctx context.Context) (*oidc.Provider, error) {
	c.mu.Do(func() {
		p, err := oidc.NewProvider(ctx, c.issuer)
		if err != nil {
			c.err = fmt.Errorf("failed to discover OIDC provider %q: %w", c.issuer, err)
			return
		}
		c.provider = p
	})
	if c.err != nil {
		return nil, c.err
	}
	return c.provider, nil
}

// VerifyIDToken verifies the raw ID token JWT string returned by the OIDC
// provider's token endpoint. It validates the signature (using JWKS), the
// issuer, audience, expiry, and the nonce claim.
//
// On success it returns the parsed *oidc.IDToken. Callers can extract custom
// claims via token.Claims(&dst).
func VerifyIDToken(ctx context.Context, providerCache *ProviderCache, clientID, rawIDToken, expectedNonce string) (*oidc.IDToken, error) {
	provider, err := providerCache.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc verify: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: clientID,
	})

	token, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	if expectedNonce != "" && token.Nonce != expectedNonce {
		return nil, fmt.Errorf("ID token nonce mismatch: got %q, want %q", token.Nonce, expectedNonce)
	}

	return token, nil
}
