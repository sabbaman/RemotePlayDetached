FROM golang:1.26-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        gcc libc6-dev pkg-config \
        libgl1-mesa-dev libx11-dev libxcursor-dev libxrandr-dev \
        libxinerama-dev libxi-dev libxxf86vm-dev \
        gcc-mingw-w64-x86-64 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
