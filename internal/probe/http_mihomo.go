//go:build probe_mihomo

package probe

import (
	"context"
	"crypto/tls"

	mihomoadapter "github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/component/ca"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func init() { nodeHTTPBackends["mihomo"] = requestNodeHTTPMihomo }

func requestNodeHTTPMihomo(ctx context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions, payload Payload) ([]domain.NodeHTTPResult, error) {
	proxies, err := mihomoProxyMaps(&payload)
	if err != nil {
		return nil, err
	}
	skipped := rendererSkippedNodeMessages(&payload, "mihomo-proxies", len(nodes))
	if len(proxies) != len(nodes)-len(skipped) {
		return nil, domain.NewError(domain.CodeProbeInvalidTarget, "mihomo HTTP payload count does not match nodes")
	}
	proxyByNode := make([]map[string]any, len(nodes))
	proxyIndex := 0
	for i := range nodes {
		if _, ok := skipped[i]; ok {
			continue
		}
		proxyByNode[i] = proxies[proxyIndex]
		proxyIndex++
	}
	mihomoTLS, err := ca.GetTLSConfig(ca.Option{})
	if err != nil {
		return nil, domain.WrapError(domain.CodeProbeCoreStartFailed, "initialize Mihomo HTTP TLS", err)
	}
	// Mihomo's TLS fork is not assignable to crypto/tls. Empty ca.Option sets
	// only these fields, matching the existing URL-test implementation.
	tlsConfig := &tls.Config{Time: mihomoTLS.Time, RootCAs: mihomoTLS.RootCAs}
	return executeNodeHTTP(ctx, nodes, opts, skipped, func(i int, _ domain.NodeIR) (nodeHTTPRoute, error) {
		proxy, err := mihomoadapter.ParseProxy(proxyByNode[i])
		if err != nil {
			return nodeHTTPRoute{}, err
		}
		return nodeHTTPRoute{dialer: mihomoURLTestDialer{proxy: proxy}, tlsConfig: tlsConfig, close: proxy.Close}, nil
	})
}
