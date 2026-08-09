# CI Container Build Optimization Design

## Context

The CI container job currently builds `linux/amd64` and `linux/arm64` for every
pull request, `main` push, manual run, and release tag. GitHub-hosted Linux
runners are amd64, so Buildx executes the arm64 Node, pnpm, Go, and Debian
stages through QEMU. The current Dockerfile does not pin architecture-neutral
or cross-compilable build stages to `$BUILDPLATFORM`, and the workflow does not
export or restore a BuildKit cache. A normal `main` push can therefore spend
more than twenty minutes rebuilding an arm64 image that is neither loaded nor
pushed.

The workflow also uses Action versions whose JavaScript runtime is declared as
Node.js 20. GitHub now forces these Actions to Node.js 24 and emits deprecation
warnings. Current stable major versions of the official Actions used by the
workflow declare `node24` directly.

## Goals

- Make ordinary pull request, `main`, and manual CI container validation fast.
- Make release-tag dual-architecture container builds avoid QEMU for pnpm and
  Go compilation.
- Preserve the published `linux/amd64,linux/arm64` GHCR manifest and existing
  stable/prerelease tag policy.
- Reuse Docker build layers between GitHub-hosted runners.
- Remove Node.js 20 Action runtime warnings.
- Keep the existing release archives, version metadata, and host-tool build
  contracts unchanged.

## Non-goals

- Change the contents or naming of GitHub Release archives.
- Change GHCR tag selection, `latest` eligibility, or release serialization.
- Introduce native arm64 runners or a multi-runner manifest assembly workflow.
- Replace the Debian runtime image or remove its installed tools.
- Publish container images for pull requests, branch pushes, or manual runs.

## Chosen Architecture

### Event-specific platform coverage

`Resolve image metadata` will emit both image tags and target platforms:

- a pushed `v*` tag uses `linux/amd64,linux/arm64`;
- every other event uses `linux/amd64` only.

QEMU setup runs only for pushed tags. The Buildx build continues to push only
for pushed tags. Daily CI therefore validates the Dockerfile and production
amd64 image without paying for an unpublished arm64 build; release tags retain
the current two-platform publication contract.

### Native build stages and Go cross-compilation

The Dockerfile's Web and Go builder stages will use
`FROM --platform=$BUILDPLATFORM`. Web assets are architecture-independent and
are built on the native runner architecture. The Go stage receives BuildKit's
`TARGETOS` and `TARGETARCH` arguments and builds the final binary with:

```text
CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH
```

The existing `make build` path remains responsible for build tags, version
linker flags, and ruleset catalog generation. Its host-executed catalog helper
must remain on `GOHOSTOS/GOHOSTARCH`; only the final Sandrone binary targets the
image architecture.

The final Debian stage remains target-platform-specific. Its package install
may still use QEMU for arm64, but dependency installation and both expensive
compilation stages no longer do. This keeps the runtime image contract intact
while removing the dominant emulation cost.

### BuildKit cache

The build-push step will restore and export a named GitHub Actions cache:

```yaml
cache-from: type=gha,scope=sandrone-container
cache-to: type=gha,mode=max,scope=sandrone-container
```

The stable scope lets normal default-branch builds warm layers later reused by
release builds. Cache misses remain correct; they only affect duration.

### Node.js 24 Action runtimes

All JavaScript Actions in the workflow will use their current Node.js 24 major
versions:

- `actions/checkout@v7`;
- `actions/setup-go@v7`;
- `actions/setup-node@v7`;
- `pnpm/action-setup@v6`;
- `docker/setup-qemu-action@v4`;
- `docker/setup-buildx-action@v4`;
- `docker/login-action@v4`;
- `docker/build-push-action@v7`.

`golangci/golangci-lint-action@v9` already declares Node.js 24 and stays on its
current major version.

## Data Flow

1. CI resolves the canonical version, tags, and event-specific platform list.
2. Non-tag runs skip QEMU and build one native amd64 image without publishing.
3. Tag runs enable QEMU for the remaining target-specific runtime operations
   and request both amd64 and arm64 outputs.
4. BuildKit runs Web compilation once on `$BUILDPLATFORM` and cross-compiles the
   Go binary for each `$TARGETARCH`.
5. Build layers are restored from and exported to the named GitHub cache.
6. Tag builds push the unchanged multi-platform GHCR manifest; the release job
   then publishes the unchanged binary archives and checksums.

## Contract and Documentation Updates

The build metadata contract test will lock:

- event-specific platform output;
- tag-only QEMU setup;
- native builder stages and target-specific Go environment;
- BuildKit cache configuration;
- Node.js 24 Action major versions;
- unchanged tag-only push and dual-architecture release behavior.

The build reference will state that ordinary CI validates amd64 only, release
tags publish both architectures, and build stages use native execution plus Go
cross-compilation.

## Verification

- Run the focused build metadata contract test first.
- Run Dockerfile static/contract checks through the repository test suite.
- Run `make lint` and `make check`.
- Run a local amd64 container build when Docker is available.
- Validate the workflow diff contains no remaining Node.js 20 Action majors.
- The first pushed branch run proves the ordinary amd64 path and cache export;
  the next run demonstrates cache reuse. A release tag remains the authoritative
  proof of the dual-architecture publish path.

## Failure Handling

- Cache restore/export failures must not alter produced artifacts; a cold build
  remains valid.
- Cross-compilation failure stops the image build before publication.
- A tag build must never silently fall back to amd64-only output.
- A non-tag build must never log in to GHCR or push the `:ci` tag.
