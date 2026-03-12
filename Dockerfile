# Build stage
FROM golang:1.24-alpine3.21 AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/forward-mcp cmd/server/main.go

# Final stage - pinned to Alpine 3.21 for zlib security fixes
FROM alpine:3.21

# Upgrade all packages to pick up security patches (zlib >= 1.3.1-r4)
RUN apk upgrade --no-cache

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/forward-mcp .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./forward-mcp"] 