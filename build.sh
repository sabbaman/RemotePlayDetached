#!/bin/bash
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd "$root"

name=${1:-RemotePlayDetached}
if [[ "$name" == */* || "$name" == . || "$name" == .. ]]; then
    printf 'The binary name must not contain a directory path.\n' >&2
    exit 1
fi
if [[ $# -gt 2 || ( -n "${2:-}" && "$2" != strip ) ]]; then
    printf 'Usage: %s [binary-name] [strip]\n' "$0" >&2
    exit 1
fi

build_binaries() {
    local flags=()
    if [[ "${2:-}" == strip ]]; then
        flags=(-ldflags=-s)
    fi
    mkdir -p build/fyne-cross/bin/{linux-amd64,windows-amd64}
    printf 'Building Linux amd64...\n'
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=gcc go build "${flags[@]}" \
        -o "build/fyne-cross/bin/linux-amd64/$name" .
    printf 'Building Windows amd64...\n'
    CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build "${flags[@]}" \
        -o "build/fyne-cross/bin/windows-amd64/$name.exe" .
}

# The container invokes this script again, but must never start nested Docker.
if [[ "${RPD_BUILD_CONTAINER:-}" == 1 ]]; then
    build_binaries "$@"
    exit 0
fi

missing=()
if [[ "$(uname -s)" != Linux || "$(uname -m)" != x86_64 ]]; then
    missing+=("Linux amd64 host")
fi
for tool in go gcc pkg-config x86_64-w64-mingw32-gcc; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        missing+=("$tool")
    fi
done
if command -v pkg-config >/dev/null 2>&1; then
    for library in gl x11 xcursor xrandr xinerama xi xxf86vm; do
        if ! pkg-config --exists "$library"; then
            missing+=("$library development libraries")
        fi
    done
fi

if [[ ${#missing[@]} == 0 ]]; then
    build_binaries "$@"
else
    printf 'Missing local build dependencies: %s\n' "${missing[*]}"
    if ! command -v docker >/dev/null 2>&1; then
        printf 'Install Docker (and start its daemon), or install the missing dependencies.\n' >&2
        exit 1
    fi
    if ! docker info >/dev/null 2>&1; then
        printf 'Docker is not accessible. Start its daemon and check your permissions.\n' >&2
        exit 1
    fi
    image=remoteplaydetached-builder:local
    printf 'Building dependency container...\n'
    docker build --platform linux/amd64 -t "$image" "$root/resources/buildContext"
    container_args=(--user "$(id -u):$(id -g)")
    if [[ "$(docker --version)" == *[Pp]odman* ]]; then
        # Keep host ownership usable inside rootless Podman's user namespace.
        if [[ "$(docker info --format '{{.Host.Security.Rootless}}')" == true ]]; then
            container_args+=(--userns=keep-id)
        fi
        container_args+=(--volume "$root:/workspace:Z")
    else
        container_args+=(--mount "type=bind,source=$root,target=/workspace")
    fi
    printf 'Compiling in the container...\n'
    docker run --rm --platform linux/amd64 \
        "${container_args[@]}" \
        -e RPD_BUILD_CONTAINER=1 -e HOME=/tmp -e GOPATH=/tmp/go -e GOCACHE=/tmp/go-cache \
        --workdir /workspace "$image" bash ./build.sh "$@"
fi

printf 'Binaries are in build/fyne-cross/bin/{linux-amd64,windows-amd64}/\n'
