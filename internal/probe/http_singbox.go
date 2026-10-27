//go:build probe_singbox

package probe

import (
	"context"
	"crypto/tls"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strings"

	box "github.com/sagernet/sing-box"
	boxadapter "github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/sing/service"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func init() { nodeHTTPBackends["sing-box"] = requestNodeHTTPSingBox }

func requestNodeHTTPSingBox(ctx context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions, payload Payload) ([]domain.NodeHTTPResult, error) {
	// HTTP consumes node renderer output, not an arbitrary sing-box runtime
	// configuration. Global listeners/services would collide across instances.
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(payload.Body, &fields); err != nil || fields == nil {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, "invalid sing-box HTTP node payload")
	}
	for field := range fields {
		if field != "outbounds" && field != "endpoints" {
			return nil, domain.NewError(domain.CodeProbeInvalidTarget, "sing-box HTTP payload must contain only node outbounds and endpoints")
		}
	}
	skipped := rendererSkippedNodeMessages(&payload, "sing-box-outbounds", len(nodes))
	byNode, err := singBoxHTTPNodePayloads(nodes, fields, skipped)
	if err != nil {
		return nil, err
	}
	return executeNodeHTTP(ctx, nodes, opts, skipped, func(requestCtx context.Context, index int, node domain.NodeIR) (nodeHTTPRoute, error) {
		// Force a separate registry even if the caller already carries one.
		boxCtx, cancel := context.WithCancel(include.Context(service.ContextWithRegistry(requestCtx, service.NewRegistry())))
		owned := true
		defer func() {
			if owned {
				cancel()
			}
		}()
		selected, err := singBoxOptions(boxCtx, &Payload{Body: byNode[index]})
		if err != nil {
			return nodeHTTPRoute{}, err
		}
		for _, outbound := range selected.Outbounds {
			if dialer, ok := outbound.Options.(option.DialerOptionsWrapper); ok && dialer.TakeDialerOptions().Detour != "" {
				return nodeHTTPRoute{}, errors.New("node HTTP detour is unsupported")
			}
		}
		for _, endpoint := range selected.Endpoints {
			if dialer, ok := endpoint.Options.(option.DialerOptionsWrapper); ok && dialer.TakeDialerOptions().Detour != "" {
				return nodeHTTPRoute{}, errors.New("node HTTP detour is unsupported")
			}
		}
		instance, err := box.New(box.Options{Context: boxCtx, Options: selected})
		if err != nil {
			return nodeHTTPRoute{}, domain.WrapError(domain.CodeProbeCoreStartFailed, "create sing-box HTTP instance", err)
		}
		closeInstance := func() error {
			cancel()
			return instance.Close()
		}
		defer func() {
			if owned {
				_ = closeInstance()
			}
		}()
		if err := instance.Start(); err != nil {
			return nodeHTTPRoute{}, domain.WrapError(domain.CodeProbeCoreStartFailed, "start sing-box HTTP instance", err)
		}
		outbound, ok := instance.Outbound().Outbound(node.Name)
		if !ok {
			return nodeHTTPRoute{}, errors.New("node outbound was not found")
		}
		tlsConfig := &tls.Config{Time: ntp.TimeFuncFromContext(boxCtx), RootCAs: boxadapter.RootPoolFromContext(boxCtx)}
		owned = false
		return nodeHTTPRoute{dialer: singBoxURLTestDialer{outbound: outbound}, tlsConfig: tlsConfig, close: closeInstance}, nil
	})
}

func singBoxHTTPNodePayloads(nodes []domain.NodeIR, fields map[string]jsontext.Value, skipped map[int]string) ([][]byte, error) {
	invalid := func() error {
		return domain.NewError(domain.CodeProbeInvalidTarget, "sing-box HTTP payload tags do not match nodes")
	}
	byTag := make(map[string][]byte, len(nodes))
	for field, value := range fields {
		var entries []jsontext.Value
		if !strings.HasPrefix(strings.TrimSpace(string(value)), "[") || json.Unmarshal(value, &entries) != nil {
			return nil, domain.NewError(domain.CodeProbeInvalidTarget, "sing-box HTTP node payload requires arrays")
		}
		for _, entry := range entries {
			var envelope struct {
				Tag string `json:"tag"`
			}
			if err := json.Unmarshal(entry, &envelope); err != nil || envelope.Tag == "" {
				return nil, invalid()
			}
			if _, exists := byTag[envelope.Tag]; exists {
				return nil, invalid()
			}
			// Preserve the opaque node config until its own worker decodes it;
			// typed option errors must not prevent other nodes from running.
			body, err := json.Marshal(map[string][]jsontext.Value{field: {entry}})
			if err != nil {
				return nil, domain.WrapError(domain.CodeProbeInvalidTarget, "encode sing-box HTTP node payload", err)
			}
			byTag[envelope.Tag] = body
		}
	}
	result := make([][]byte, len(nodes))
	seen := make(map[string]bool, len(nodes))
	for i, node := range nodes {
		if node.Name == "" || seen[node.Name] {
			return nil, invalid()
		}
		seen[node.Name] = true
		if _, skip := skipped[i]; skip {
			continue
		}
		selected, ok := byTag[node.Name]
		if !ok {
			return nil, invalid()
		}
		result[i] = selected
		delete(byTag, node.Name)
	}
	if len(byTag) != 0 {
		return nil, invalid()
	}
	return result, nil
}
