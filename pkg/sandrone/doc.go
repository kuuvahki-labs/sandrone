// Package sandrone exposes the embeddable public API for the Sandrone
// node conversion engine.
//
// Use New for the default in-memory engine, NewWithFS to back resources with an
// afero filesystem, or NewWithStore to provide an application-owned storage
// boundary. Engine methods cover the main service flows: Parse, Render,
// Convert, Diagnose, RenderSubscription, GetFile, Inspect and resource
// persistence.
//
// Public request, result and model types are aliases to the internal domain
// model so callers can build structured requests without importing internal
// packages. Architecture and API details live in docs/architecture/overview.md
// and docs/reference/http-api/README.md.
package sandrone
