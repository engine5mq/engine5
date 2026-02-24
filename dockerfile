# syntax=docker/dockerfile:1

FROM golang:1.21.0-alpine AS builder

# Install security updates and required tools
RUN apk update && apk add --no-cache \
    ca-certificates \
    git \
    tzdata && \
    apk upgrade

# Create app user for security
RUN adduser -D -g '' engine5user
RUN apk --no-cache add ca-certificates
# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Verify dependencies
RUN go mod verify

# Copy the source code
COPY *.go ./

# Build with security flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/engine5 \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o /engine5

# Production stage
FROM scratch

# Copy CA certificates for TLS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy user info for non-root execution
COPY --from=builder /etc/passwd /etc/passwd

# Copy the binary
COPY --from=builder /engine5 /engine5

# Create directory for certificates
# RUN mkdir -p /app/certs

# Use non-root user
USER engine5user

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/engine5", "--health-check"] || exit 1

# Document ports - change as needed
EXPOSE 3535

# Run the binary
CMD ["/engine5"]
# End of Dockerfile
