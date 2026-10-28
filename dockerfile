# syntax=docker/dockerfile:1

## Argümanlar
# Arch: ARM ya da x86 (buildx tarafından --platform ile otomatik set edilir)
# VERSION: image sürümü (release.env -> IMAGE_TAG, build-push-docker.sh tarafından --build-arg ile geçilir)
#
## Target'lar
#   docker buildx build --target engine5 ...  -> broker (varsayılan, son stage)
#   docker buildx build --target aes ...      -> AES servisi

# BUILDPLATFORM: derlemenin yapıldığı host platformu (örn. linux/amd64)
# TARGETOS / TARGETARCH: hedeflenen platform (örn. linux/arm64), buildx tarafından otomatik doldurulur
FROM --platform=$BUILDPLATFORM golang:1.21.0-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

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
COPY cmd ./cmd
COPY internal ./internal

# Build with security flags (TARGETOS/TARGETARCH buildx tarafından set edilir, örn: linux/amd64, linux/arm64, linux/arm/v7)
RUN for app in engine5 aes; do \
        CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build \
        -a -installsuffix cgo \
        -ldflags='-w -s -extldflags "-static"' \
        -o /out/${app} ./cmd/${app} || exit 1; \
    done

# Ortak çalışma zamanı katmanı
FROM scratch AS runtime-base

ARG VERSION=dev
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/engine5mq/engine5"

# Copy CA certificates for TLS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy user info for non-root execution
COPY --from=builder /etc/passwd /etc/passwd

# AES stage
FROM runtime-base AS aes

COPY --from=builder /out/aes /aes

# Varsayılan kurallar; production'da /config/aes.rules.json volume ile override edilmeli
COPY --chown=engine5user aes.rules.example.json /config/aes.rules.json
ENV E5_AES_RULES_FILE=/config/aes.rules.json

USER engine5user

CMD ["/aes"]

# Engine5 (broker) stage - target belirtilmezse bu image üretilir
FROM runtime-base AS engine5

# Copy the binary
COPY --from=builder /out/engine5 /engine5

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
