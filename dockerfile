# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.21.0-alpine AS builder

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY *.go ./

# Build the binary with optimizations for size
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o engine5 .

# Final stage - use scratch for minimal size
FROM scratch

# Copy ca-certificates for HTTPS connections
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/engine5 /engine5

# Optional:
# To bind to a TCP port, runtime parameters must be supplied to the docker command.
# But we can document in the Dockerfile what ports
# the application is going to listen on by default.
# https://docs.docker.com/engine/reference/builder/#expose
# EXPOSE 8080

# Run
CMD ["/engine5"]