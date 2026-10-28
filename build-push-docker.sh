#!/usr/bin/env bash
set -euo pipefail

# Kullanım:
#   ./build-push-docker.sh                 # release.env'deki IMAGE_TAG ile engine5 + aes
#   ./build-push-docker.sh 0.0.14-alpha    # tag'i parametre ile override et
#   TARGETS="aes" ./build-push-docker.sh   # sadece aes
#
# Ortam değişkenleri ile override edilebilir: IMAGE_TAG, TARGETS, PLATFORMS, ENGINE5_IMAGE, AES_IMAGE, PUSH
# linux/arm/v7 : Raspberry Pi (32-bit, ör. Pi 2/3/Zero 2)
# linux/arm64  : Raspberry Pi (64-bit) ve Apple Silicon (M1/M2/M3) üzerinde Docker/Rosetta ile

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

# Ortamdan gelen değerler release.env'den önceliklidir
_override_tag="${1:-${IMAGE_TAG:-}}"
_override_platforms="${PLATFORMS:-}"
_override_engine5_image="${ENGINE5_IMAGE:-}"
_override_aes_image="${AES_IMAGE:-}"

# shellcheck source=release.env
set -a
. ./release.env
set +a

export IMAGE_TAG="${_override_tag:-${IMAGE_TAG}}"
export PLATFORMS="${_override_platforms:-${PLATFORMS}}"
export ENGINE5_IMAGE="${_override_engine5_image:-${ENGINE5_IMAGE}}"
export AES_IMAGE="${_override_aes_image:-${AES_IMAGE}}"
TARGETS="${TARGETS:-engine5 aes}"
PUSH="${PUSH:-true}"
DOCKER_FILE="./dockerfile"
BUILDER_NAME="engine5-builder"

if [[ -z "${IMAGE_TAG}" ]]; then
    echo "IMAGE_TAG boş olamaz (release.env veya parametre)" >&2
    exit 1
fi

# Multi-arch build için buildx builder yoksa oluştur ve kullan
if ! docker buildx inspect "${BUILDER_NAME}" >/dev/null 2>&1; then
    docker buildx create --name "${BUILDER_NAME}" --use
else
    docker buildx use "${BUILDER_NAME}"
fi

if [[ "${PUSH}" == "true" ]]; then
    OUTPUT_FLAG="--push"
else
    # Push olmadan sadece derleme doğrulaması (multi-arch imajlar yerel docker'a yüklenemez)
    OUTPUT_FLAG="--output=type=cacheonly"
fi

for target in ${TARGETS//,/ }; do
    case "${target}" in
        engine5) image="${ENGINE5_IMAGE}" ;;
        aes)     image="${AES_IMAGE}" ;;
        *) echo "Bilinmeyen target: ${target} (engine5 | aes)" >&2; exit 1 ;;
    esac

    echo ">> ${target}: ${image}:${IMAGE_TAG} [${PLATFORMS}]"
    docker buildx build \
        --platform "${PLATFORMS}" \
        --file "${DOCKER_FILE}" \
        --target "${target}" \
        --build-arg "VERSION=${IMAGE_TAG}" \
        -t "${image}:${IMAGE_TAG}" \
        ${OUTPUT_FLAG} \
        .
done
