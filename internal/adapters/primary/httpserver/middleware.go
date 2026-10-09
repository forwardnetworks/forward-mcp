package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/forward-mcp/internal/ports"
	"github.com/rs/cors"
	"golang.org/x/time/rate"
)

// CORSMiddleware creates a CORS middleware
func CORSMiddleware(cfg *ports.HTTPConfig) *cors.Cors {
	return cors.New(cors.Options{
		AllowedOrigins: cfg.CORSOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Request-ID",
		},
		ExposedHeaders: []string{
			"X-Request-ID",
		},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	})
}

// RateLimiter implements per-user rate limiting using token bucket algorithm
type RateLimiter struct {
	limiters  map[string]*rate.Limiter
	mu        sync.RWMutex
	rate      rate.Limit
	burst     int
	perMinute int
	log       ports.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int, log ports.Logger) *RateLimiter {
	// Convert requests per minute to rate.Limit (per second)
	r := rate.Limit(float64(requestsPerMinute) / 60.0)
	burst := requestsPerMinute / 4 // 25% of per-minute limit as burst

	if burst < 1 {
		burst = 1
	}

	return &RateLimiter{
		limiters:  make(map[string]*rate.Limiter),
		rate:      r,
		burst:     burst,
		perMinute: requestsPerMinute,
		log:       log,
	}
}

// getLimiter gets or creates a limiter for a user
func (rl *RateLimiter) getLimiter(userID string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[userID]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := rl.limiters[userID]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[userID] = limiter
	return limiter
}

// Middleware returns an HTTP middleware that enforces rate limiting
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context (set by auth middleware)
			user, ok := GetUserFromContext(r.Context())
			if !ok {
				// No user in context - allow (auth middleware should have blocked)
				next.ServeHTTP(w, r)
				return
			}

			// Get limiter for this user
			limiter := rl.getLimiter(user.UserID)

			// Check if request is allowed
			if !limiter.Allow() {
				rl.log.Debug("Rate limit exceeded for user: %s", user.Username)
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.perMinute))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(log ports.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Auth runs deeper in the chain and fills this slot, so the log
			// line can name the user.
			slot := &userSlot{}
			next.ServeHTTP(wrapped, r.WithContext(context.WithValue(r.Context(), userSlotKey, slot)))

			duration := time.Since(start)

			username := "anonymous"
			if slot.user != nil {
				username = slot.user.Username
			}

			log.Info("HTTP %s %s %d %s user=%s",
				r.Method,
				r.URL.Path,
				wrapped.statusCode,
				duration,
				username,
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush passes through so streamed MCP events reach the client immediately.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// HSTS: Force HTTPS for 2 years
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")

			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Content Security Policy
			w.Header().Set("Content-Security-Policy", "default-src 'self'")

			// Disable caching for sensitive endpoints
			if r.URL.Path != "/health" && r.URL.Path != "/ready" && r.URL.Path != "/metrics" {
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
				w.Header().Set("Pragma", "no-cache")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ConnectionLimit caps concurrent requests (including open streams) to the
// wrapped handler. Requests beyond the cap get 503 instead of queueing.
func ConnectionLimit(max int, log ports.Logger) func(http.Handler) http.Handler {
	slots := make(chan struct{}, max)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
				next.ServeHTTP(w, r)
			default:
				log.Warn("Connection limit reached (%d); rejecting %s %s", max, r.Method, r.URL.Path)
				w.Header().Set("Retry-After", "5")
				http.Error(w, "Too many open connections, retry shortly", http.StatusServiceUnavailable)
			}
		})
	}
}
