//go:build probe_singbox

package probe

import (
	"context"
	"crypto/tls"
	"errors"

	boxadapter "github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing/common/ntp"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func init() { nodeHTTPBackends["sing-box"] = requestNodeHTTPSingBox }

func requestNodeHTTPSingBox(ctx context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions, payload Payload) ([]domain.NodeHTTPResult, error) {
	boxCtx, cancel := context.WithCancel(include.Context(ctx))
	defer cancel()
	options, err := singBoxOptions(boxCtx, &payload)
	if err != nil {
		return nil, err
	}
	instance, err := startSingBox(boxCtx, options)
	if err != nil {
		return nil, err
	}
	defer func() { _ = instance.Close() }()
	tlsConfig := &tls.Config{Time: ntp.TimeFuncFromContext(boxCtx), RootCAs: boxadapter.RootPoolFromContext(boxCtx)}
	skipped := rendererSkippedNodeMessages(&payload, "sing-box-outbounds", len(nodes))
	return executeNodeHTTP(ctx, nodes, opts, skipped, func(_ int, node domain.NodeIR) (nodeHTTPRoute, error) {
		outbound, ok := instance.Outbound().Outbound(node.Name)
		if !ok {
			return nodeHTTPRoute{}, errors.New("node outbound was not found")
		}
		return nodeHTTPRoute{dialer: singBoxURLTestDialer{outbound: outbound}, tlsConfig: tlsConfig}, nil
	})
}
