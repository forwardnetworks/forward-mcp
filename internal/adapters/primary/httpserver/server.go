package httpserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/forward-mcp/internal/domain"
	"github.com/forward-mcp/internal/ports"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 30 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

// Server represents the HTTP/SSE server
type Server struct {
	config      *domain.HTTPConfig
	log         ports.Logger
	httpServer  *http.Server
	rateLimiter *RateLimiter
}

// New creates a new HTTP/SSE server
func New(cfg *domain.HTTPConfig, log ports.Logger) *Server {
	return &Server{
		config:      cfg,
		log:         log,
		rateLimiter: NewRateLimiter(cfg.RateLimit, log),
	}
}

// Start starts the HTTP server with MCP SSE handler
func (s *Server) Start(ctx context.Context, mcpServer *mcp.Server) error {
	if !s.config.Enabled {
		return fmt.Errorf("HTTP server is not enabled")
	}

	// Create HTTP router
	mux := http.NewServeMux()

	// Health and metrics endpoints (no auth required)
	mux.HandleFunc("/health", HealthHandler("4.0.0"))
	mux.HandleFunc("/ready", ReadinessHandler("4.0.0"))

	// Create MCP SSE handler
	// The handler creates a new server instance for each connection
	// This allows per-connection state and authentication
	sseHandler := mcp.NewSSEHandler(
		func(r *http.Request) *mcp.Server {
			// Return the provided MCP server
			// In the future, this could create per-user servers based on auth
			return mcpServer
		},
		&mcp.SSEOptions{},
	)

	// Register SSE endpoint with authentication and rate limiting
	sseEndpoint := s.applyMiddleware(sseHandler, true)
	mux.Handle("/sse", sseEndpoint)

	// Build final handler chain
	handler := s.buildHandlerChain(mux)

	// Configure HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	readTimeout := time.Duration(s.config.ReadTimeout) * time.Second
	if readTimeout == 0 {
		readTimeout = defaultReadTimeout
	}

	writeTimeout := time.Duration(s.config.WriteTimeout) * time.Second
	if writeTimeout == 0 {
		writeTimeout = defaultWriteTimeout
	}

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  defaultIdleTimeout,
		BaseContext: func(listener net.Listener) context.Context {
			return ctx
		},
	}

	// Configure TLS if certificates are provided
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		s.log.Info("Starting HTTPS server on %s (TLS 1.3+)", addr)

		// Configure TLS with strict security settings
		s.httpServer.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
			PreferServerCipherSuites: true,
		}

		// Start HTTPS server
		go func() {
			if err := s.httpServer.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile); err != nil && err != http.ErrServerClosed {
				s.log.Error("HTTPS server error: %v", err)
			}
		}()
	} else {
		// SECURITY: Require explicit opt-in for insecure HTTP
		// This should NEVER be used in production
		if !s.config.AllowInsecure {
			return fmt.Errorf("SECURITY ERROR: TLS certificates required. Set FORWARD_HTTP_TLS_CERT and FORWARD_HTTP_TLS_KEY, " +
				"or set FORWARD_HTTP_ALLOW_INSECURE=true for development only")
		}

		s.log.Warn("SECURITY WARNING: Starting HTTP server on %s without TLS - DEVELOPMENT ONLY, NEVER USE IN PRODUCTION", addr)

		// Start HTTP server (insecure, development only)
		go func() {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.log.Error("HTTP server error: %v", err)
			}
		}()
	}

	s.log.Info("HTTP/SSE server started successfully")
	return nil
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	s.log.Info("Stopping HTTP server...")

	// Attempt graceful shutdown
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.log.Error("HTTP server shutdown error: %v", err)
		// Force close if graceful shutdown fails
		return s.httpServer.Close()
	}

	s.log.Info("HTTP server stopped")
	return nil
}

// buildHandlerChain builds the middleware chain for all requests
func (s *Server) buildHandlerChain(handler http.Handler) http.Handler {
	// Apply middleware in order (innermost to outermost)
	// Order matters: logging should be outermost to capture everything

	// Security headers (innermost)
	handler = SecurityHeadersMiddleware()(handler)

	// CORS
	if len(s.config.CORSOrigins) > 0 {
		corsMiddleware := CORSMiddleware(s.config)
		handler = corsMiddleware.Handler(handler)
	}

	// Logging (outermost - logs everything)
	handler = LoggingMiddleware(s.log)(handler)

	return handler
}

// applyMiddleware applies authentication and rate limiting to an endpoint
func (s *Server) applyMiddleware(handler http.Handler, requireAuth bool) http.Handler {
	// Apply rate limiting first (after auth)
	if s.config.RateLimit > 0 {
		handler = s.rateLimiter.Middleware()(handler)
	}

	// Apply authentication if required
	if requireAuth {
		handler = AuthMiddleware(s.config, s.log)(handler)
	}

	return handler
}
