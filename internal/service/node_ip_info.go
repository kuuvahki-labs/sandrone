package service

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
	"github.com/kuuvahki-labs/sandrone/internal/iplookup"
)

const ipLookupTimeout = 5 * time.Second

func (s *Service) InspectNode(ctx context.Context, req domain.NodeInspectRequest) (*domain.NodeInspectResult, error) {
	if len(req.Include) == 0 {
		return nil, domain.NewError(domain.CodeInvalidArgument, "include must contain at least one node information field")
	}
	seen := make(map[domain.NodeInspectField]struct{}, len(req.Include))
	for _, field := range req.Include {
		if field != domain.NodeInspectURI && field != domain.NodeInspectIP {
			return nil, domain.NewError(domain.CodeInvalidArgument, "include contains an unsupported node information field")
		}
		if _, exists := seen[field]; exists {
			return nil, domain.NewError(domain.CodeInvalidArgument, "include must not contain duplicate node information fields")
		}
		seen[field] = struct{}{}
	}
	result := &domain.NodeInspectResult{}
	if _, included := seen[domain.NodeInspectURI]; included {
		uri, err := s.inspectNodeURI(ctx, req.Node)
		if err != nil {
			return nil, err
		}
		result.URI = uri
	}
	if _, included := seen[domain.NodeInspectIP]; included {
		ip, err := s.lookupNodeIPInfo(ctx, req.Node.Server)
		if err != nil {
			return nil, err
		}
		result.IP = ip
	}
	return result, nil
}

func (s *Service) inspectNodeURI(ctx context.Context, node domain.NodeIR) (*domain.NodeURIInfo, error) {
	rendered, err := s.render(ctx, domain.RenderRequest{
		Format: "uri-list",
		Target: "uri-list",
		Nodes:  []domain.NodeIR{node},
	})
	if err != nil {
		return nil, err
	}
	uri := strings.TrimSpace(string(rendered.Body))
	if uri == "" || strings.ContainsAny(uri, "\r\n") || rendered.Report.Render.SuccessCount != 1 {
		return nil, domain.NewError(domain.CodeRenderFailed, "node did not render to exactly one URI")
	}
	return &domain.NodeURIInfo{Value: uri, Warnings: rendered.Report.Warnings}, nil
}

func (s *Service) lookupNodeIPInfo(ctx context.Context, rawServer string) (*domain.NodeIPInfoResult, error) {
	server := strings.TrimSpace(rawServer)
	parsed, literal, err := parseNodeServer(server)
	if err != nil {
		return nil, err
	}
	if !literal {
		parsed, err = s.resolveNodeServer(ctx, server)
		if err != nil {
			return nil, err
		}
	}
	result := &domain.NodeIPInfoResult{
		Server:    server,
		IP:        parsed.String(),
		IPVersion: ipVersion(parsed),
		Public:    fetcher.IsPublicAddress(parsed),
	}
	if !result.Public {
		return result, nil
	}
	attribution, err := s.lookupIPAttribution(ctx, parsed)
	if err != nil {
		return nil, err
	}
	result.CountryCode = attribution.CountryCode
	result.Country = attribution.Country
	result.ContinentCode = attribution.ContinentCode
	result.Continent = attribution.Continent
	result.ASN = attribution.ASN
	result.ASName = attribution.ASName
	result.ASDomain = attribution.ASDomain
	result.Source = &domain.NodeIPInfoSource{Name: "ipwho.is", URL: "https://ipwho.is"}
	return result, nil
}

func (s *Service) resolveNodeServer(ctx context.Context, server string) (netip.Addr, error) {
	resolveCtx, cancel := context.WithTimeout(ctx, ipLookupTimeout)
	defer cancel()
	resolved, err := s.ipResolver.LookupIPAddr(resolveCtx, server)
	if err != nil {
		return netip.Addr{}, domain.WrapError(domain.CodeIPLookupFailed, "resolve node server", err)
	}
	seen := map[netip.Addr]struct{}{}
	addresses := make([]netip.Addr, 0, len(resolved))
	for _, candidate := range resolved {
		ip, ok := netip.AddrFromSlice(candidate.IP)
		if !ok {
			return netip.Addr{}, domain.NewError(domain.CodeIPLookupFailed, "node server resolved to an invalid address")
		}
		ip = ip.Unmap()
		if _, duplicate := seen[ip]; duplicate {
			continue
		}
		seen[ip] = struct{}{}
		addresses = append(addresses, ip)
	}
	if len(addresses) == 0 {
		return netip.Addr{}, domain.NewError(domain.CodeIPLookupFailed, "node server resolved to no addresses")
	}
	return addresses[0], nil
}

func (s *Service) lookupIPAttribution(ctx context.Context, ip netip.Addr) (iplookup.Attribution, error) {
	attribution, err := s.ipLookup.Lookup(ctx, ip)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return iplookup.Attribution{}, err
		}
		return iplookup.Attribution{}, domain.WrapError(domain.CodeIPLookupFailed, "look up IP attribution", err)
	}
	return attribution, nil
}

func parseNodeServer(server string) (netip.Addr, bool, error) {
	if server == "" {
		return netip.Addr{}, false, domain.NewError(domain.CodeInvalidArgument, "server is required")
	}
	literal := server
	if inner, hasOpen := strings.CutPrefix(literal, "["); hasOpen {
		var hasClose bool
		literal, hasClose = strings.CutSuffix(inner, "]")
		if !hasClose {
			return netip.Addr{}, false, domain.NewError(domain.CodeInvalidArgument, "server must be an IP address or DNS name")
		}
	} else if strings.HasSuffix(literal, "]") {
		return netip.Addr{}, false, domain.NewError(domain.CodeInvalidArgument, "server must be an IP address or DNS name")
	}
	if ip, err := netip.ParseAddr(literal); err == nil {
		if ip.Zone() != "" {
			return netip.Addr{}, false, domain.NewError(domain.CodeInvalidArgument, "server IP address must not include a zone")
		}
		return ip.Unmap(), true, nil
	}
	if !validDNSName(server) {
		return netip.Addr{}, false, domain.NewError(domain.CodeInvalidArgument, "server must be an IP address or DNS name")
	}
	return netip.Addr{}, false, nil
}

func validDNSName(server string) bool {
	name := strings.TrimSuffix(server, ".")
	if name == "" || len(name) > 253 {
		return false
	}
	for label := range strings.SplitSeq(name, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
				(char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func ipVersion(ip netip.Addr) int {
	if ip.Is4() {
		return 4
	}
	return 6
}
