# Build stage. CGO is required: the SQLite driver (mattn/go-sqlite3) is C.
FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/forward-mcp ./cmd/server

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --create-home --uid 10001 app

WORKDIR /app
COPY --from=builder /out/forward-mcp /app/forward-mcp
# The bundled NQE query library is the offline fallback for query search.
COPY spec/ /app/spec/
RUN mkdir -p /app/data /home/app/.forward-mcp/data && chown -R app:app /app /home/app

USER app
ENV HOME=/home/app \
    FORWARD_HTTP_ENABLED=true \
    FORWARD_HTTP_HOST=0.0.0.0 \
    FORWARD_HTTP_PORT=8080

# Databases, bloom indexes and caches. Mount volumes here to keep them.
VOLUME ["/home/app/.forward-mcp", "/app/data"]

EXPOSE 8080

# Remote server mode. Provide TLS (FORWARD_HTTP_TLS_CERT/KEY) or, behind a
# TLS-terminating proxy, FORWARD_HTTP_ALLOW_INSECURE=true. Probe /health.
ENTRYPOINT ["/app/forward-mcp"]
