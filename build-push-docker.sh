#!/usr/bin/env bash
set -euo pipefail

export IMAGE_NAME='hcangunduz/engine5'
export IMAGE_TAG='0.0.13-alpha'
export DOCKER_FILE="./dockerfile"

# Hedef platformlar: virgülle ayrılmış liste. Ortam değişkeni ile override edilebilir:
#   PLATFORMS="linux/amd64,linux/arm64" ./build-push-docker.sh
# linux/arm/v7 : Raspberry Pi (32-bit, ör. Pi 2/3/Zero 2)
# linux/arm64  : Raspberry Pi (64-bit) ve Apple Silicon (M1/M2/M3) üzerinde Docker/Rosetta ile
export PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64,linux/arm/v7}"

BUILDER_NAME="engine5-builder"

# Multi-arch build için buildx builder yoksa oluştur ve kullan
if ! docker buildx inspect "${BUILDER_NAME}" >/dev/null 2>&1; then
    docker buildx create --name "${BUILDER_NAME}" --use
else
    docker buildx use "${BUILDER_NAME}"
fi

docker buildx build \
    --platform "${PLATFORMS}" \
    --file "${DOCKER_FILE}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    --push \
    .

