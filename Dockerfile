# syntax=docker/dockerfile:1
# SPDX-License-Identifier: AGPL-3.0-only
#
# Container images for cmd/my (ADR-0010): multi-arch (linux/amd64, linux/arm64), static, non-root, distroless.
#
#   docker build -t my .                       # the server (default target "app")
#   docker build -t my-schema --target schema . # Atlas Community Edition + schema/*.sql (ADR-0023)
#
# Base images are pinned by tag and multi-arch index digest. The Go tag must match .tool-versions.

ARG GO_IMAGE=golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

# Cross-compiles on the build host's platform; Go needs no emulation for the target architecture.
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETOS TARGETARCH
# Version stamped into `my version` and NodeInfo.
ARG VERSION=dev
ENV GOTOOLCHAIN=local CGO_ENABLED=0
WORKDIR /src
RUN --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download
RUN --mount=type=bind,target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/my ./cmd/my \
    && mkdir -p /out/data

# Atlas Community Edition for the target platform, at the version and checksum pinned in scripts/install-atlas.sh.
FROM ${GO_IMAGE} AS atlas
RUN --mount=type=bind,source=scripts/install-atlas.sh,target=/install-atlas.sh \
    /install-atlas.sh /out/atlas

# One-shot schema apply: `atlas schema apply` with schema/<dialect>.sql under /schema (ADR-0023).
FROM ${RUNTIME_IMAGE} AS schema
COPY --from=atlas /out/atlas /usr/local/bin/atlas
COPY schema/*.sql /schema/
# Owned by the non-root user so that a named volume mounted here starts out writable.
COPY --from=build --chown=nonroot:nonroot /out/data /data
WORKDIR /data
ENTRYPOINT ["/usr/local/bin/atlas"]
CMD ["version"]

FROM ${RUNTIME_IMAGE} AS app
COPY --from=build /out/my /usr/local/bin/my
COPY --from=build --chown=nonroot:nonroot /out/data /data
WORKDIR /data
ENV MY_LISTEN=:8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/my"]
CMD ["serve"]
