package httpserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/forward-mcp/internal/ports"
)

// contextKey is a private type for context keys to avoid collisions
type contextKey string

const (
	userContextKey contextKey = "user"
)

// UserInfo represents an authenticated user
type UserInfo struct {
	Username string
	UserID   string
	Email    string
}

// AuthMiddleware creates an authentication middleware based on config
func AuthMiddleware(cfg *ports.HTTPConfig, log ports.Logger) func(http.Handler) http.Handler {
	switch cfg.AuthMode {
	case "jwt":
		return jwtAuthMiddleware(cfg, log)
	case "api-key":
		return apiKeyAuthMiddleware(cfg, log)
	case "none":
		log.Warn("HTTP server running with no authentication - use only for development")
		return noAuthMiddleware()
	default:
		log.Error("Unknown auth mode: %s, defaulting to api-key", cfg.AuthMode)
		return apiKeyAuthMiddleware(cfg, log)
	}
}

// jwtAuthMiddleware validates JWT tokens using JWKS
func jwtAuthMiddleware(cfg *ports.HTTPConfig, log ports.Logger) func(http.Handler) http.Handler {
	// Create JWKS cache for public key management
	var jwksCache *JWKSCache
	if cfg.JWTPublicKeyURL != "" {
		jwksCache = NewJWKSCache(cfg.JWTPublicKeyURL, log)
		jwksCache.client = jwksHTTPClient
		// Fetch keys once at startup so the first request is fast. After that,
		// GetKeySet refreshes them when they are older than the TTL.
		go func() {
			if _, err := jwksCache.GetKeySet(context.Background()); err != nil {
				log.Warn("Initial JWKS fetch failed; will retry on first request: %v", err)
			}
		}()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Debug("Missing Authorization header")
				http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
				return
			}

			// Check Bearer prefix
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Debug("Invalid Authorization header format")
				http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Validate JWT using JWKS if configured
			if jwksCache != nil {
				token, err := jwksCache.ValidateToken(r.Context(), tokenString, cfg.JWTIssuer, cfg.JWTAudience)
				if err != nil {
					log.Debug("JWT validation failed: %v", err)
					http.Error(w, "Invalid token", http.StatusUnauthorized)
					return
				}

				// Extract user info from validated token
				sub, ok := token.Subject()
				if !ok || sub == "" {
					log.Debug("JWT token missing subject")
					http.Error(w, "Invalid token: missing subject", http.StatusUnauthorized)
					return
				}

				user := &UserInfo{
					UserID:   sub,
					Username: sub,
				}

				// Add user to context
				ctx := withUser(r.Context(), user)
				log.Debug("JWT authenticated: user=%s (sub=%s)", user.Username, user.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				// No JWKS URL configured
				log.Debug("JWT authentication requested but JWTPublicKeyURL not configured")
				http.Error(w, "JWT authentication not configured", http.StatusInternalServerError)
			}
		})
	}
}

// apiKeyAuthMiddleware validates API keys
func apiKeyAuthMiddleware(cfg *ports.HTTPConfig, log ports.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Debug("Missing Authorization header")
				http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
				return
			}

			// Check Bearer prefix
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Debug("Invalid Authorization header format")
				http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
				return
			}

			apiKey := parts[1]

			// Validate API key
			username, ok := cfg.APIKeys[apiKey]
			if !ok {
				log.Debug("Invalid API key")
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Create user info
			user := &UserInfo{
				Username: username,
				UserID:   username,
			}

			// Add user to context
			ctx := withUser(r.Context(), user)
			log.Debug("API key authenticated: user=%s", username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// noAuthMiddleware allows all requests (development only)
func noAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create anonymous user
			user := &UserInfo{
				Username: "anonymous",
				UserID:   "anonymous",
			}

			ctx := withUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext extracts user info from request context
func GetUserFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value(userContextKey).(*UserInfo)
	return user, ok
}

// userSlot lets the outer logging middleware see who auth let in.
type userSlot struct{ user *UserInfo }

const userSlotKey contextKey = "user-slot"

// withUser stores the user in the context and fills the logging slot if present.
func withUser(ctx context.Context, user *UserInfo) context.Context {
	if slot, ok := ctx.Value(userSlotKey).(*userSlot); ok {
		slot.user = user
	}
	return context.WithValue(ctx, userContextKey, user)
}
