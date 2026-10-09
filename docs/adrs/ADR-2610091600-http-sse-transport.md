# ADR-2610091600: HTTP/SSE Transport for Remote MCP Server

**Status:** Proposed  
**Date:** 2026-10-09  
**Context:** Hexagonal architecture (ADR-2610091315)

---

## Context

Forward-MCP currently runs as a stdio-based CLI tool, requiring it to execute on the same machine as the MCP client (Claude Desktop, Claude Code). Users want to deploy forward-mcp as a centralized service that multiple clients can access remotely.

**Use cases:**
- Deploy forward-mcp on a dedicated server (keeps Forward API credentials centralized)
- Multiple users share one instance (resource pooling)
- Run heavy operations on server hardware (not laptops)
- Easier updates (update server once, not N clients)
- Cloud deployments (Docker, Kubernetes)

## Decision

Implement HTTP/SSE transport as a new primary adapter alongside the existing stdio adapter.

### Transport Layer

Use MCP Go SDK's built-in SSE (Server-Sent Events) support:

```go
// Primary adapter: internal/adapters/primary/httpserver/
handler := mcp.NewSSEHandler(
    func(r *http.Request) *mcp.Server { 
        return server // authenticated, per-request server instance
    },
    &mcp.SSEOptions{},
)

http.Handle("/sse", authMiddleware(handler))
```

**Why SSE:**
- Official MCP protocol standard for HTTP transport
- Bidirectional communication (server can push notifications)
- Built into MCP Go SDK v1.7.0
- Works through firewalls/proxies better than WebSocket

### Authentication

**OAuth 2.0 Bearer tokens with JWT:**

```
Authorization: Bearer eyJhbGciOiJSUzI1Ni...
```

**Token validation:**
- Support multiple JWT issuers (Auth0, Okta, custom)
- Validate signature with public key (RS256)
- Check expiration, audience, issuer
- Extract user identity for audit logs

**Configuration:**
```bash
FORWARD_HTTP_AUTH_MODE=jwt
FORWARD_HTTP_JWT_ISSUER=https://auth.example.com
FORWARD_HTTP_JWT_AUDIENCE=forward-mcp
FORWARD_HTTP_JWT_PUBLIC_KEY_URL=https://auth.example.com/.well-known/jwks.json
```

**Fallback: API Key mode** (simpler, for service-to-service):
```bash
FORWARD_HTTP_AUTH_MODE=api-key
FORWARD_HTTP_API_KEYS=key1:user1,key2:user2
```

### HTTP Server

**Standards:**
- HTTP/2 (automatic HTTP/1.1 fallback)
- TLS 1.3+ (we already enforce this)
- CORS headers for web clients
- Security headers (HSTS, X-Content-Type-Options, etc.)
- Rate limiting (per-user, token bucket)

**Endpoints:**
```
POST   /sse                 - SSE endpoint for MCP protocol
GET    /health              - Health check (returns 200 if healthy)
GET    /ready               - Readiness check (returns 200 if ready to serve)
GET    /metrics             - Prometheus metrics
GET    /.well-known/mcp     - MCP server metadata
```

### Observability

**OpenTelemetry:**
- Distributed tracing (trace tool calls end-to-end)
- Metrics (request count, latency, errors)
- Context propagation (W3C Trace Context)

**Structured Logging:**
- JSON format
- Correlation IDs for request tracking
- User identity in logs (for audit)

**Metrics (Prometheus format):**
```
forward_mcp_http_requests_total{method,path,status}
forward_mcp_http_request_duration_seconds{method,path}
forward_mcp_tool_calls_total{tool,status}
forward_mcp_tool_duration_seconds{tool}
forward_mcp_auth_failures_total{reason}
```

### Configuration

**12-Factor App Principles:**
- All config via environment variables
- Secrets via env vars or mounted files (Kubernetes secrets)
- No hardcoded defaults for sensitive values

```bash
# Server
FORWARD_HTTP_ENABLED=true
FORWARD_HTTP_PORT=8080
FORWARD_HTTP_TLS_CERT=/etc/certs/tls.crt
FORWARD_HTTP_TLS_KEY=/etc/certs/tls.key

# Authentication
FORWARD_HTTP_AUTH_MODE=jwt|api-key|none
FORWARD_HTTP_JWT_ISSUER=https://auth.example.com
FORWARD_HTTP_JWT_AUDIENCE=forward-mcp
FORWARD_HTTP_JWT_PUBLIC_KEY_URL=https://auth.example.com/.well-known/jwks.json

# API Key mode (fallback)
FORWARD_HTTP_API_KEYS=key1:user1,key2:user2

# Security
FORWARD_HTTP_CORS_ORIGINS=https://app.example.com
FORWARD_HTTP_RATE_LIMIT_PER_USER=100  # requests per minute
FORWARD_HTTP_MAX_CONCURRENT_CONNECTIONS=100

# Observability
FORWARD_OTEL_ENABLED=true
FORWARD_OTEL_ENDPOINT=http://otel-collector:4318
FORWARD_OTEL_SERVICE_NAME=forward-mcp
FORWARD_LOG_FORMAT=json
```

### Dual-Mode Operation

**Same binary supports both transports:**

```bash
# Stdio mode (existing, default)
./forward-mcp

# HTTP mode
./forward-mcp --http --port 8080

# Both modes simultaneously
./forward-mcp --http --port 8080 --stdio
```

Command-line flags override environment variables.

### Security Considerations

**Rate Limiting:**
- Per-user token bucket (100 req/min default)
- Per-IP backup limit (1000 req/min)
- Websocket connection limits (100 concurrent)

**CORS:**
- Configurable allowed origins
- Credentials support (Authorization header)
- Preflight caching

**Security Headers:**
```
Strict-Transport-Security: max-age=63072000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Content-Security-Policy: default-src 'self'
```

**TLS:**
- Minimum TLS 1.3
- Secure cipher suites only (same as Forward API client)
- Certificate validation
- Optional mTLS (mutual TLS) for client certificates

## Architecture

### New Components

**Primary Adapter: `internal/adapters/primary/httpserver/`**
```
httpserver/
├── server.go           - HTTP server setup
├── sse_handler.go      - MCP SSE handler
├── auth.go             - Authentication middleware
├── middleware.go       - Logging, CORS, rate limiting
├── metrics.go          - Prometheus metrics
├── health.go           - Health/readiness checks
└── server_test.go
```

**Configuration: `internal/domain/config.go`**
```go
type HTTPConfig struct {
    Enabled         bool
    Port            int
    TLSCertFile     string
    TLSKeyFile      string
    AuthMode        string // "jwt", "api-key", "none"
    JWTIssuer       string
    JWTAudience     string
    JWTPublicKeyURL string
    APIKeys         map[string]string // key -> user
    CORSOrigins     []string
    RateLimit       int // per user per minute
    MaxConnections  int
}
```

**Dependencies to Add:**
```go
// go.mod additions
require (
    github.com/golang-jwt/jwt/v5 v5.2.0         // JWT parsing
    github.com/lestrrat-go/jwx/v2 v2.0.21       // JWKS (public key rotation)
    github.com/rs/cors v1.10.1                   // CORS middleware
    golang.org/x/time v0.5.0                     // Rate limiting
    go.opentelemetry.io/otel v1.24.0            // OpenTelemetry
    go.opentelemetry.io/otel/exporters/prometheus v0.46.0
    go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0
)
```

### Hexagonal Architecture Fit

```
┌─────────────────────────────────────┐
│   MCP Clients (Remote)              │
│   - Claude Desktop                  │
│   - Claude Code                     │
│   - Web Apps                        │
└──────────┬──────────────────────────┘
           │ HTTP/SSE
┌──────────▼──────────────────────────┐
│ Primary Adapter: HTTP/SSE Server    │  ← NEW
│  - SSE handler                      │
│  - Authentication middleware        │
│  - Rate limiting                    │
│  - Metrics/tracing                  │
└──────────┬──────────────────────────┘
           │
┌──────────▼──────────────────────────┐
│    Use Cases Layer                  │
│    (unchanged)                      │
└─────────────────────────────────────┘
```

**Zero changes to:**
- Use cases (business logic)
- Domain types
- Ports (interfaces)
- Secondary adapters (API client, storage, cache)

**New files only:**
- `internal/adapters/primary/httpserver/*.go`
- `cmd/server/main.go` (add HTTP mode flag)

### Client Configuration

**Claude Desktop (`claude_desktop_config.json`):**
```json
{
  "mcpServers": {
    "forward-mcp": {
      "transport": "sse",
      "url": "https://forward-mcp.example.com/sse",
      "headers": {
        "Authorization": "Bearer eyJhbGciOiJSUzI1Ni..."
      }
    }
  }
}
```

**curl testing:**
```bash
# Health check
curl https://forward-mcp.example.com/health

# SSE connection
curl -N -H "Authorization: Bearer $TOKEN" \
  https://forward-mcp.example.com/sse
```

## Deployment

### Docker

**Dockerfile:**
```dockerfile
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o forward-mcp ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=builder /app/forward-mcp /usr/local/bin/
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
USER nobody
ENTRYPOINT ["forward-mcp"]
CMD ["--http", "--port", "8080"]
```

**Docker Compose:**
```yaml
version: '3.8'
services:
  forward-mcp:
    build: .
    ports:
      - "8080:8080"
    environment:
      FORWARD_API_BASE_URL: ${FORWARD_API_BASE_URL}
      FORWARD_API_KEY: ${FORWARD_API_KEY}
      FORWARD_API_SECRET: ${FORWARD_API_SECRET}
      FORWARD_HTTP_ENABLED: "true"
      FORWARD_HTTP_AUTH_MODE: jwt
      FORWARD_HTTP_JWT_ISSUER: https://auth.example.com
      FORWARD_HTTP_JWT_AUDIENCE: forward-mcp
      FORWARD_OTEL_ENABLED: "true"
    volumes:
      - mcp-data:/data
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
    restart: unless-stopped

volumes:
  mcp-data:
```

### Kubernetes

**Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: forward-mcp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: forward-mcp
  template:
    metadata:
      labels:
        app: forward-mcp
    spec:
      containers:
      - name: forward-mcp
        image: forward-mcp:4.1.0
        args: ["--http", "--port", "8080"]
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: FORWARD_API_BASE_URL
          valueFrom:
            secretKeyRef:
              name: forward-api
              key: base-url
        - name: FORWARD_API_KEY
          valueFrom:
            secretKeyRef:
              name: forward-api
              key: api-key
        - name: FORWARD_API_SECRET
          valueFrom:
            secretKeyRef:
              name: forward-api
              key: api-secret
        - name: FORWARD_HTTP_ENABLED
          value: "true"
        - name: FORWARD_HTTP_AUTH_MODE
          value: "jwt"
        - name: FORWARD_HTTP_JWT_ISSUER
          value: "https://auth.example.com"
        - name: FORWARD_OTEL_ENDPOINT
          value: "http://otel-collector:4318"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
---
apiVersion: v1
kind: Service
metadata:
  name: forward-mcp
spec:
  selector:
    app: forward-mcp
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: forward-mcp
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - forward-mcp.example.com
    secretName: forward-mcp-tls
  rules:
  - host: forward-mcp.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: forward-mcp
            port:
              number: 80
```

## Implementation Plan

### Phase 1: Core HTTP/SSE (1 week)
1. Add HTTP server with SSE handler
2. Basic authentication (API key mode)
3. Health/ready endpoints
4. TLS support
5. Integration tests

### Phase 2: Security & Auth (3 days)
1. JWT authentication
2. JWKS public key rotation
3. Rate limiting
4. CORS configuration
5. Security headers

### Phase 3: Observability (3 days)
1. OpenTelemetry tracing
2. Prometheus metrics
3. Structured JSON logging
4. Correlation IDs

### Phase 4: Deployment (2 days)
1. Dockerfile
2. Docker Compose example
3. Kubernetes manifests
4. Documentation

### Phase 5: Testing & Docs (2 days)
1. End-to-end tests
2. Load testing
3. Security audit
4. Updated README
5. Deployment guide

**Total: ~2.5 weeks**

## Consequences

### Positive

- **Remote deployment**: Run on dedicated server hardware
- **Multi-user**: Multiple clients share one instance
- **Security**: Centralized credential management
- **Scalability**: Horizontal scaling with load balancer
- **Standard protocols**: SSE, JWT, OAuth 2.0, OpenTelemetry
- **Cloud-ready**: Docker + Kubernetes deployment
- **Zero disruption**: Existing stdio mode unchanged

### Negative

- **Complexity**: More deployment options to document
- **Security surface**: HTTP endpoints need careful security
- **Operations**: Need monitoring, alerting, incident response
- **Dependencies**: More libraries (JWT, OTEL, CORS)

### Risks & Mitigations

**Risk: Authentication bypass**
- Mitigation: Comprehensive auth tests, security audit

**Risk: Rate limiting bypass**
- Mitigation: Multi-layer limits (per-user, per-IP, global)

**Risk: Token leakage**
- Mitigation: Short token expiration, token rotation support

**Risk: DDoS**
- Mitigation: Rate limiting, connection limits, deploy behind CDN

## Alternatives Considered

### 1. WebSocket Instead of SSE
- **Rejected**: SSE is the MCP standard, simpler, works better through proxies

### 2. gRPC Transport
- **Rejected**: Not part of MCP spec, more complex client setup

### 3. Plain HTTP REST (no SSE)
- **Rejected**: Loses bidirectional communication, not MCP standard

### 4. Separate HTTP Binary
- **Rejected**: Prefer single binary with mode flag (simpler distribution)

## References

- MCP Protocol Specification: https://modelcontextprotocol.io/
- MCP Go SDK: https://github.com/modelcontextprotocol/go-sdk
- Server-Sent Events: https://html.spec.whatwg.org/multipage/server-sent-events.html
- OAuth 2.0: https://datatracker.ietf.org/doc/html/rfc6749
- JWT: https://datatracker.ietf.org/doc/html/rfc7519
- OpenTelemetry: https://opentelemetry.io/
- 12-Factor App: https://12factor.net/
- ADR-2610091315: Hexagonal Architecture
