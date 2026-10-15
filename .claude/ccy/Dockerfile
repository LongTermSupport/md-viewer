# md-viewer project container
#
# Extends claude-yolo with what md-viewer's development needs: the pinned Go toolchain, a C
# compiler for cgo and the race detector, Gio's Linux build headers, and a virtual display for
# the --time smoke run. The base image is Debian 12 (bookworm); apt packages follow that
# release and its security updates, while every downloaded tool is pinned by version and
# sha256.

ARG BASE_IMAGE=claude-yolo:latest
FROM ${BASE_IMAGE}

# Build and test tools.
# - gcc, libc6-dev: Gio's window code is cgo, and `go test -race` needs cgo.
# - pkg-config: Gio's cgo files find their libraries with `#cgo linux pkg-config:`.
# - git, curl, wget, ca-certificates, xz-utils: fetching go-qa-ci and the release archives
#   go-qa downloads and unpacks.
# - jq: reading `go list -json` and GitHub API output while working.
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        gcc \
        libc6-dev \
        pkg-config \
        git \
        curl \
        wget \
        ca-certificates \
        xz-utils \
        jq \
    && rm -rf /var/lib/apt/lists/*

# Gio v0.10.3's Linux build dependencies. Gio's install page
# (https://gioui.org/doc/install/linux) lists, for Debian and Ubuntu: gcc pkg-config
# libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-x11-dev libgles2-mesa-dev
# libegl1-mesa-dev libffi-dev libxcursor-dev libvulkan-dev. The list below is that list with
# bookworm's current names (libegl-dev and libgles-dev replace the transitional *-mesa-dev
# packages), plus the libraries Gio's cgo lines name directly, checked against the v0.10.3
# source:
# - libwayland-dev: pkg-config wayland-client, wayland-cursor, wayland-egl (app/os_wayland.go,
#   app/wayland_*.go).
# - libxkbcommon-dev: pkg-config xkbcommon (app/internal/xkb).
# - libxkbcommon-x11-dev, libx11-dev, libx11-xcb-dev, libxcursor-dev, libxfixes-dev:
#   pkg-config x11 xkbcommon-x11 x11-xcb xcursor xfixes (app/os_x11.go).
# - libegl-dev: pkg-config egl (internal/egl).
# - libgles-dev: the GLES headers Gio's OpenGL backend compiles against.
# - libvulkan-dev: the Vulkan headers internal/vk compiles against.
# - libffi-dev: on Gio's list; Wayland's client library is built on it.
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        libwayland-dev \
        libxkbcommon-dev \
        libxkbcommon-x11-dev \
        libx11-dev \
        libx11-xcb-dev \
        libxcursor-dev \
        libxfixes-dev \
        libegl-dev \
        libgles-dev \
        libvulkan-dev \
        libffi-dev \
    && rm -rf /var/lib/apt/lists/*

# A virtual display, so the built binary can be run in the container even though there is no
# real one: `xvfb-run -a md-viewer --time testdata/elements.md` (docs/MILESTONES.md). Its
# numbers are information only; speed targets are judged on the owner's desktop (SPEC SPEED).
# - xvfb, xauth: xvfb-run starts an X server and needs xauth for its cookie.
# - libegl-mesa0, libgl1-mesa-dri: Mesa's EGL and its software renderer, which Gio's X11
#   window uses for drawing when there is no GPU.
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        xvfb \
        xauth \
        libegl-mesa0 \
        libgl1-mesa-dri \
    && rm -rf /var/lib/apt/lists/*

# GitHub CLI: pushes, pull requests, and the authenticated clone of go-qa-ci while it is
# private (git and go install use `gh auth setup-git` for its credentials).
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update \
    && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# Go, pinned. Debian 12's golang is 1.19; md-viewer's go.mod says go 1.26.9, which is also
# what go-qa-ci's go.mod requires. Bump GO_VERSION and GO_SHA256 together with go.mod
# (sha256 from https://go.dev/dl/).
ARG GO_VERSION=1.26.9
ARG GO_SHA256=42d158b4d8f7b61ac0a830567c940a86098fb7aac52e467a5ebec03ef5cc2f8d
RUN wget -qO /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    && echo "${GO_SHA256}  /tmp/go.tgz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz \
    && ln -s /usr/local/go/bin/go /usr/local/bin/go \
    && ln -s /usr/local/go/bin/gofmt /usr/local/bin/gofmt
# GOTOOLCHAIN=local: go.mod must never make go download another toolchain.
ENV GOTOOLCHAIN=local

# go-qa-ci's tools (go-qa, golangci-lint, ast-grep, deadcode, govulncheck, gremlins) are NOT
# installed here. The gate is the plain Go toolchain above; go-qa is an add-on that installs
# its own checksum-verified tools from go-qa-ci's tools.lock. Copying those pins into this
# file would make a second source that drifts.

# Verify the toolchain, and that every pkg-config module Gio's cgo lines name resolves.
RUN go version \
    && gcc --version \
    && pkg-config --exists --print-errors \
        wayland-client wayland-cursor wayland-egl egl xkbcommon xkbcommon-x11 \
        x11 x11-xcb xcursor xfixes \
    && command -v xvfb-run \
    && gh --version
