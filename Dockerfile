# syntax=docker/dockerfile:1

FROM node:24.17.0-bookworm AS web
WORKDIR /src/web

ARG NPM_REGISTRY=""
ARG PNPM_REGISTRY=""

RUN if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi \
  && npm install -g pnpm@11.5.2

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN if [ -n "$PNPM_REGISTRY" ]; then pnpm config set registry "$PNPM_REGISTRY"; elif [ -n "$NPM_REGISTRY" ]; then pnpm config set registry "$NPM_REGISTRY"; fi \
  && pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm build

FROM golang:1.25.11-bookworm AS build
WORKDIR /src

ARG GOPROXY=""

COPY go.mod go.sum ./
RUN if [ -n "$GOPROXY" ]; then go env -w GOPROXY="$GOPROXY"; fi \
  && go mod download

ARG VERSION="dev"
ARG REVISION=""
RUN if [ -z "$REVISION" ] && [ "$VERSION" != "dev" ]; then \
    printf '%s\n' 'VERSION requires a complete REVISION; use VERSION=dev for untraceable builds' >&2; \
    exit 1; \
  fi

COPY . .
COPY --from=web /src/web/build/client ./internal/entry/webui/static
RUN make build BUILD_BIN=/out/sandrone VERSION="$VERSION" REVISION="$REVISION"

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --create-home --home-dir /app --shell /usr/sbin/nologin sandrone \
  && mkdir -p /app/data \
  && chown -R sandrone:sandrone /app

COPY --from=build /out/sandrone /usr/local/bin/sandrone
COPY --from=web --chown=sandrone:sandrone /src/web/build/client /app/static

USER sandrone
WORKDIR /app
VOLUME ["/app/data"]
EXPOSE 1137

ARG VERSION="dev"
ARG REVISION=""
LABEL org.opencontainers.image.version=$VERSION \
  org.opencontainers.image.revision=$REVISION \
  org.opencontainers.image.source=https://github.com/kuuvahki-labs/sandrone

ENTRYPOINT ["sandrone"]
CMD ["serve", "http", "--listen", "0.0.0.0:1137", "--data-dir", "/app/data"]
