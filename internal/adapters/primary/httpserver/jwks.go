package httpserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/forward-mcp/internal/ports"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// JWKSCache caches JWKS public keys with automatic refresh
type JWKSCache struct {
	url         string
	cache       jwk.Set
	mu          sync.RWMutex
	log         ports.Logger
	lastRefresh time.Time
	refreshTTL  time.Duration
}

// NewJWKSCache creates a new JWKS cache with automatic refresh
func NewJWKSCache(url string, log ports.Logger) *JWKSCache {
	return &JWKSCache{
		url:        url,
		log:        log,
		refreshTTL: 1 * time.Hour, // Refresh public keys hourly
	}
}

// GetKeySet returns the cached key set, refreshing if needed
func (j *JWKSCache) GetKeySet(ctx context.Context) (jwk.Set, error) {
	j.mu.RLock()
	needsRefresh := j.cache == nil || time.Since(j.lastRefresh) > j.refreshTTL
	j.mu.RUnlock()

	if needsRefresh {
		if err := j.refresh(ctx); err != nil {
			// If refresh fails but we have cached keys, use them
			j.mu.RLock()
			cached := j.cache
			j.mu.RUnlock()

			if cached != nil {
				j.log.Warn("JWKS refresh failed, using cached keys: %v", err)
				return cached, nil
			}
			return nil, fmt.Errorf("failed to fetch JWKS and no cached keys available: %w", err)
		}
	}

	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.cache, nil
}

// refresh fetches the latest JWKS from the URL
func (j *JWKSCache) refresh(ctx context.Context) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Double-check after acquiring lock
	if j.cache != nil && time.Since(j.lastRefresh) <= j.refreshTTL {
		return nil
	}

	j.log.Debug("Refreshing JWKS from %s", j.url)

	// Fetch with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	keySet, err := jwk.Fetch(fetchCtx, j.url)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	j.cache = keySet
	j.lastRefresh = time.Now()

	j.log.Debug("JWKS refreshed successfully (%d keys)", keySet.Len())
	return nil
}

// ValidateToken validates a JWT token using the cached JWKS
func (j *JWKSCache) ValidateToken(ctx context.Context, tokenString string, issuer, audience string) (jwt.Token, error) {
	// Get current key set
	keySet, err := j.GetKeySet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}

	// Parse and validate token
	token, err := jwt.Parse(
		[]byte(tokenString),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
	)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	return token, nil
}

// StartBackgroundRefresh starts a background goroutine to refresh JWKS periodically
func (j *JWKSCache) StartBackgroundRefresh(ctx context.Context) {
	ticker := time.NewTicker(j.refreshTTL)
	defer ticker.Stop()

	// Initial refresh
	if err := j.refresh(ctx); err != nil {
		j.log.Error("Initial JWKS refresh failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			j.log.Debug("JWKS background refresh stopped")
			return
		case <-ticker.C:
			if err := j.refresh(ctx); err != nil {
				j.log.Error("JWKS refresh failed: %v", err)
			}
		}
	}
}
