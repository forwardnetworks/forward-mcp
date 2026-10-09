package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/forward-mcp/internal/domain"
	"github.com/forward-mcp/internal/ports"
	"github.com/golang-jwt/jwt/v5"
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
	Claims   jwt.MapClaims
}

// AuthMiddleware creates an authentication middleware based on config
func AuthMiddleware(cfg *domain.HTTPConfig, log ports.Logger) func(http.Handler) http.Handler {
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

// jwtAuthMiddleware validates JWT tokens
func jwtAuthMiddleware(cfg *domain.HTTPConfig, log ports.Logger) func(http.Handler) http.Handler {
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

			// Parse and validate JWT
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Validate signing method
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}

				// TODO: Fetch public key from JWKS URL (cfg.JWTPublicKeyURL)
				// For now, this is a placeholder. In production, implement JWKS fetching
				// using github.com/lestrrat-go/jwx/v2/jwk
				return nil, fmt.Errorf("JWT public key validation not yet implemented")
			})

			if err != nil {
				log.Debug("JWT validation failed: %v", err)
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			if !token.Valid {
				log.Debug("JWT token is not valid")
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Extract claims
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				log.Debug("JWT claims extraction failed")
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			// Validate issuer
			if cfg.JWTIssuer != "" {
				if iss, ok := claims["iss"].(string); !ok || iss != cfg.JWTIssuer {
					log.Debug("JWT issuer mismatch: expected %s, got %s", cfg.JWTIssuer, iss)
					http.Error(w, "Invalid token issuer", http.StatusUnauthorized)
					return
				}
			}

			// Validate audience
			if cfg.JWTAudience != "" {
				if aud, ok := claims["aud"].(string); !ok || aud != cfg.JWTAudience {
					log.Debug("JWT audience mismatch: expected %s, got %s", cfg.JWTAudience, aud)
					http.Error(w, "Invalid token audience", http.StatusUnauthorized)
					return
				}
			}

			// Extract user info
			user := &UserInfo{
				Claims: claims,
			}

			if sub, ok := claims["sub"].(string); ok {
				user.UserID = sub
			}
			if name, ok := claims["name"].(string); ok {
				user.Username = name
			} else if email, ok := claims["email"].(string); ok {
				user.Username = email
			} else {
				user.Username = user.UserID
			}

			// Add user to context
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// apiKeyAuthMiddleware validates API keys
func apiKeyAuthMiddleware(cfg *domain.HTTPConfig, log ports.Logger) func(http.Handler) http.Handler {
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
			ctx := context.WithValue(r.Context(), userContextKey, user)
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

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext extracts user info from request context
func GetUserFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value(userContextKey).(*UserInfo)
	return user, ok
}
