package probe

import (
	"context"
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type nodeHTTPBackend func(context.Context, []domain.NodeIR, domain.NodeHTTPOptions, Payload) ([]domain.NodeHTTPResult, error)

// Backends register only at package initialization, according to build tags.
var nodeHTTPBackends = map[string]nodeHTTPBackend{}

func nodeHTTPCore(core string) string {
	if core == "" {
		return "sing-box"
	}
	return core
}

// NodeHTTPAvailable reports whether the selected HTTP core is compiled in.
// An empty core selects sing-box; it never falls back to another backend.
func NodeHTTPAvailable(core string) bool {
	_, ok := nodeHTTPBackends[nodeHTTPCore(core)]
	return ok
}

// RequestNodeHTTP receives validated options and uniquely named nodes.
// No default dialer or environment proxy is used for destination traffic.
func RequestNodeHTTP(ctx context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions, payload Payload) ([]domain.NodeHTTPResult, error) {
	opts.Core = nodeHTTPCore(opts.Core)
	backend, ok := nodeHTTPBackends[opts.Core]
	if !ok {
		return nil, domain.NewError(domain.CodeProbeBackendUnavailable, fmt.Sprintf("node HTTP core %q is not available in this build", opts.Core))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return backend(ctx, nodes, opts, payload)
}
