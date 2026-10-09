package httpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/forward-mcp/internal/ports"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second
	defaultReadHeaderTimeout = 10 * time.Second
	defaultSessionTimeout    = 30 * time.Minute
)

// MCP endpoints. /mcp is Streamable HTTP (current spec); /sse is the legacy
// HTTP+SSE transport kept for older clients.
const (
	MCPPath       = "/mcp"
	LegacySSEPath = "/sse"
)

// Server represents the HTTP server for the MCP transports
type Server struct {
	config      *ports.HTTPConfig
	log         ports.Logger
	httpServer  *http.Server
	rateLimiter *RateLimiter
}

// New creates a new HTTP server
func New(cfg *ports.HTTPConfig, log ports.Logger) *Server {
	return &Server{
		config:      cfg,
		log:         log,
		rateLimiter: NewRateLimiter(cfg.RateLimit, log),
	}
}

// Handler builds the full HTTP handler: health probes, both MCP transports,
// auth, rate limiting, CORS and security headers.
func (s *Server) Handler(mcpServer *mcp.Server) (http.Handler, error) {
	getServer := func(*http.Request) *mcp.Server { return mcpServer }

	streamable := mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
		SessionTimeout: defaultSessionTimeout,
	})
	legacySSE := mcp.NewSSEHandler(getServer, &mcp.SSEOptions{})

	// Browser cross-origin POSTs are refused unless the origin is trusted.
	csrf := http.NewCrossOriginProtection()
	for _, origin := range s.config.CORSOrigins {
		if err := csrf.AddTrustedOrigin(origin); err != nil {
			return nil, fmt.Errorf("invalid CORS origin %q: %w", origin, err)
		}
	}

	// One auth middleware for both endpoints, so JWT mode runs one JWKS cache.
	protect := s.protectMiddleware()

	writeTimeout := s.seconds(s.config.WriteTimeout, defaultWriteTimeout)

	mux := http.NewServeMux()
	mux.Handle("/health", http.TimeoutHandler(HealthHandler(ports.Version), writeTimeout, "timeout"))
	mux.Handle("/ready", http.TimeoutHandler(ReadinessHandler(ports.Version), writeTimeout, "timeout"))
	mux.Handle(MCPPath, csrf.Handler(protect(streamable)))
	mux.Handle(LegacySSEPath, csrf.Handler(protect(legacySSE)))

	return s.buildHandlerChain(mux), nil
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context, mcpServer *mcp.Server) error {
	if !s.config.Enabled {
		return fmt.Errorf("HTTP server is not enabled")
	}

	tlsEnabled := s.config.TLSCertFile != "" && s.config.TLSKeyFile != ""
	if !tlsEnabled && !s.config.AllowInsecure {
		return fmt.Errorf("SECURITY ERROR: TLS certificates required. Set FORWARD_HTTP_TLS_CERT and FORWARD_HTTP_TLS_KEY, " +
			"or set FORWARD_HTTP_ALLOW_INSECURE=true for development only")
	}

	handler, err := s.Handler(mcpServer)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       s.seconds(s.config.ReadTimeout, defaultReadTimeout),
		// No server-wide WriteTimeout: it would cut long-lived MCP streams.
		// Short endpoints get a per-route timeout in Handler instead.
		WriteTimeout: 0,
		IdleTimeout:  defaultIdleTimeout,
		BaseContext:  func(net.Listener) context.Context { return ctx },
	}

	if tlsEnabled {
		s.httpServer.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13}
		s.log.Info("Starting HTTPS server on %s (TLS 1.3+)", addr)
		go func() {
			if err := s.httpServer.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile); err != nil && err != http.ErrServerClosed {
				s.log.Error("HTTPS server error: %v", err)
			}
		}()
	} else {
		s.log.Warn("SECURITY WARNING: Starting HTTP server on %s without TLS - DEVELOPMENT ONLY, NEVER USE IN PRODUCTION", addr)
		go func() {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.log.Error("HTTP server error: %v", err)
			}
		}()
	}

	s.log.Info("MCP HTTP server started: Streamable HTTP at %s, legacy SSE at %s", MCPPath, LegacySSEPath)
	return nil
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	s.log.Info("Stopping HTTP server...")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.log.Error("HTTP server shutdown error: %v", err)
		return s.httpServer.Close()
	}
	s.log.Info("HTTP server stopped")
	return nil
}

// buildHandlerChain wraps every route with security headers, CORS and logging
func (s *Server) buildHandlerChain(handler http.Handler) http.Handler {
	handler = SecurityHeadersMiddleware()(handler)
	if len(s.config.CORSOrigins) > 0 {
		handler = CORSMiddleware(s.config).Handler(handler)
	}
	return LoggingMiddleware(s.log)(handler)
}

// protectMiddleware returns auth followed by per-user rate limiting
func (s *Server) protectMiddleware() func(http.Handler) http.Handler {
	auth := AuthMiddleware(s.config, s.log)
	return func(next http.Handler) http.Handler {
		if s.config.RateLimit > 0 {
			next = s.rateLimiter.Middleware()(next)
		}
		return auth(next)
	}
}

func (s *Server) seconds(v int, fallback time.Duration) time.Duration {
	if v <= 0 {
		return fallback
	}
	return time.Duration(v) * time.Second
}
