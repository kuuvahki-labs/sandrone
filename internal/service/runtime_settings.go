package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/fetcher"
	"github.com/kuuvahki-labs/sandrone/internal/probe"
)

const legacyDefaultUserAgent = "sandrone/0"

func DefaultRuntimeSettings() domain.RuntimeSettings {
	return domain.RuntimeSettings{
		RemoteDefaults: domain.RemoteDefaults{
			UserAgent: buildinfo.UserAgent(),
			TimeoutMS: int(fetcher.DefaultTimeout / time.Millisecond),
		},
		ProbeDefaults: domain.ProbeDefaults{
			Layer:       string(domain.ProbeLayerProtocol),
			Method:      string(domain.ProbeAuto),
			Core:        "sing-box",
			URL:         probe.URLTestTarget(domain.ProbeRequest{Method: domain.ProbeURLTest}),
			NTPServer:   probe.NTPServerFromRequest(domain.ProbeRequest{}),
			TimeoutMS:   5000,
			Attempts:    1,
			Concurrency: 10,
		},
		CacheDefaults: domain.CacheDefaults{
			RemoteFetchTTLSeconds:         0,
			SubscriptionTrafficTTLSeconds: 60,
		},
	}
}

func (s *Service) GetRuntimeSettings(ctx context.Context) (domain.RuntimeSettings, error) {
	if s.metaStore == nil {
		return DefaultRuntimeSettings(), nil
	}
	settings, err := s.metaStore.GetRuntimeSettings(ctx)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultRuntimeSettings(), nil
		}
		return domain.RuntimeSettings{}, err
	}
	return normalizeRuntimeSettings(settings)
}

func (s *Service) EffectiveRuntimeSettings(ctx context.Context) (domain.RuntimeSettings, error) {
	return s.GetRuntimeSettings(ctx)
}

func (s *Service) PutRuntimeSettings(ctx context.Context, settings domain.RuntimeSettings) error {
	if s.metaStore == nil {
		return storeUnavailable()
	}
	normalized, err := normalizeRuntimeSettings(settings)
	if err != nil {
		return err
	}
	if err := s.metaStore.PutRuntimeSettings(ctx, normalized); err != nil {
		return err
	}
	s.logResource(ctx, "put", "settings", "runtime")
	return nil
}

func (s *Service) remoteInputWithDefaults(ctx context.Context, input domain.RemoteInput) (domain.RemoteInput, error) {
	settings, err := s.EffectiveRuntimeSettings(ctx)
	if err != nil {
		return domain.RemoteInput{}, err
	}
	defaults := settings.RemoteDefaults
	out := input
	if strings.TrimSpace(out.UserAgent) == "" {
		out.UserAgent = defaults.UserAgent
	}
	if strings.TrimSpace(out.Proxy) == "" {
		out.Proxy = defaults.Proxy
	}
	if out.TimeoutMS <= 0 {
		out.TimeoutMS = defaults.TimeoutMS
	}
	if out.CacheTTLSeconds <= 0 {
		out.CacheTTLSeconds = settings.CacheDefaults.RemoteFetchTTLSeconds
	}
	return out, nil
}

func (s *Service) probeRequestWithDefaults(ctx context.Context, req domain.ProbeRequest) (domain.ProbeRequest, error) {
	settings, err := s.EffectiveRuntimeSettings(ctx)
	if err != nil {
		return domain.ProbeRequest{}, err
	}
	defaults := settings.ProbeDefaults
	out := req
	if strings.TrimSpace(string(out.Layer)) == "" {
		out.Layer = domain.ProbeLayer(defaults.Layer)
	}
	if strings.TrimSpace(string(out.Method)) == "" {
		out.Method = domain.ProbeMethod(defaults.Method)
	}
	if strings.TrimSpace(out.Core) == "" {
		out.Core = defaults.Core
	}
	if strings.TrimSpace(out.URL) == "" {
		out.URL = defaults.URL
	}
	if strings.TrimSpace(out.NTPServer) == "" {
		out.NTPServer = defaults.NTPServer
	}
	if out.TimeoutMS <= 0 {
		out.TimeoutMS = defaults.TimeoutMS
	}
	if out.Attempts <= 0 {
		out.Attempts = defaults.Attempts
	}
	if out.Concurrency <= 0 {
		out.Concurrency = defaults.Concurrency
	}
	if out.CacheTTLSeconds <= 0 {
		out.CacheTTLSeconds = defaults.CacheTTLSeconds
	}
	return out, nil
}

func normalizeRuntimeSettings(settings domain.RuntimeSettings) (domain.RuntimeSettings, error) {
	defaults := DefaultRuntimeSettings()
	out := defaults

	if settings.RemoteDefaults.TimeoutMS < 0 {
		return domain.RuntimeSettings{}, domain.NewError(domain.CodeInvalidArgument, "remote timeout_ms must be non-negative")
	}
	if settings.ProbeDefaults.TimeoutMS < 0 {
		return domain.RuntimeSettings{}, domain.NewError(domain.CodeInvalidArgument, "probe timeout_ms must be non-negative")
	}
	if settings.ProbeDefaults.Attempts < 0 {
		return domain.RuntimeSettings{}, domain.NewError(domain.CodeInvalidArgument, "probe attempts must be non-negative")
	}
	if settings.ProbeDefaults.Concurrency < 0 {
		return domain.RuntimeSettings{}, domain.NewError(domain.CodeInvalidArgument, "probe concurrency must be non-negative")
	}
	if settings.ProbeDefaults.CacheTTLSeconds < 0 {
		return domain.RuntimeSettings{}, domain.NewError(domain.CodeInvalidArgument, "probe cache_ttl_seconds must be non-negative")
	}
	if settings.CacheDefaults.RemoteFetchTTLSeconds < 0 {
		return domain.RuntimeSettings{}, domain.NewError(domain.CodeInvalidArgument, "remote_fetch_ttl_seconds must be non-negative")
	}
	if settings.CacheDefaults.SubscriptionTrafficTTLSeconds < 0 {
		return domain.RuntimeSettings{}, domain.NewError(domain.CodeInvalidArgument, "subscription_traffic_ttl_seconds must be non-negative")
	}

	if value := strings.TrimSpace(settings.RemoteDefaults.UserAgent); value != "" {
		if value == legacyDefaultUserAgent {
			value = buildinfo.UserAgent()
		}
		out.RemoteDefaults.UserAgent = value
	}
	if value := strings.TrimSpace(settings.RemoteDefaults.Proxy); value != "" {
		if err := validateRuntimeProxyURL(value); err != nil {
			return domain.RuntimeSettings{}, err
		}
		out.RemoteDefaults.Proxy = value
	}
	if settings.RemoteDefaults.TimeoutMS > 0 {
		out.RemoteDefaults.TimeoutMS = settings.RemoteDefaults.TimeoutMS
	}

	if value := strings.TrimSpace(settings.ProbeDefaults.Layer); value != "" {
		out.ProbeDefaults.Layer = string(probe.NormalizeLayer(domain.ProbeLayer(value)))
	}
	if value := strings.TrimSpace(settings.ProbeDefaults.Method); value != "" {
		out.ProbeDefaults.Method = string(probe.NormalizeMethod(domain.ProbeMethod(value)))
	}
	if value := strings.TrimSpace(settings.ProbeDefaults.Core); value != "" {
		out.ProbeDefaults.Core = probe.NormalizeCore(value)
	}
	if value := strings.TrimSpace(settings.ProbeDefaults.URL); value != "" {
		if err := validateRuntimeHTTPURL("probe url", value); err != nil {
			return domain.RuntimeSettings{}, err
		}
		out.ProbeDefaults.URL = value
	}
	if value := strings.TrimSpace(settings.ProbeDefaults.NTPServer); value != "" {
		out.ProbeDefaults.NTPServer = value
	}
	if settings.ProbeDefaults.TimeoutMS > 0 {
		out.ProbeDefaults.TimeoutMS = settings.ProbeDefaults.TimeoutMS
	}
	if settings.ProbeDefaults.Attempts > 0 {
		out.ProbeDefaults.Attempts = settings.ProbeDefaults.Attempts
	}
	if settings.ProbeDefaults.Concurrency > 0 {
		out.ProbeDefaults.Concurrency = settings.ProbeDefaults.Concurrency
	}
	if settings.ProbeDefaults.CacheTTLSeconds > 0 {
		out.ProbeDefaults.CacheTTLSeconds = settings.ProbeDefaults.CacheTTLSeconds
	}
	if settings.CacheDefaultsSpecified() {
		out.CacheDefaults = settings.CacheDefaults
	}
	if err := validateRuntimeSettings(out); err != nil {
		return domain.RuntimeSettings{}, err
	}
	return out, nil
}

func validateRuntimeSettings(settings domain.RuntimeSettings) error {
	if settings.RemoteDefaults.TimeoutMS <= 0 {
		return domain.NewError(domain.CodeInvalidArgument, "remote timeout_ms must be positive")
	}
	switch settings.ProbeDefaults.Layer {
	case string(domain.ProbeLayerProtocol), string(domain.ProbeLayerProxy):
	default:
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported probe layer %q", settings.ProbeDefaults.Layer))
	}
	switch settings.ProbeDefaults.Method {
	case string(domain.ProbeAuto), string(domain.ProbeTCPConnect), string(domain.ProbeUDPNTP), string(domain.ProbeURLTest):
	default:
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported probe method %q", settings.ProbeDefaults.Method))
	}
	switch settings.ProbeDefaults.Core {
	case "mihomo", "sing-box":
	default:
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported probe core %q", settings.ProbeDefaults.Core))
	}
	if settings.ProbeDefaults.TimeoutMS <= 0 {
		return domain.NewError(domain.CodeInvalidArgument, "probe timeout_ms must be positive")
	}
	if settings.ProbeDefaults.Attempts <= 0 {
		return domain.NewError(domain.CodeInvalidArgument, "probe attempts must be positive")
	}
	if settings.ProbeDefaults.Concurrency <= 0 {
		return domain.NewError(domain.CodeInvalidArgument, "probe concurrency must be positive")
	}
	if settings.ProbeDefaults.CacheTTLSeconds < 0 {
		return domain.NewError(domain.CodeInvalidArgument, "probe cache_ttl_seconds must be non-negative")
	}
	if settings.CacheDefaults.RemoteFetchTTLSeconds < 0 {
		return domain.NewError(domain.CodeInvalidArgument, "remote_fetch_ttl_seconds must be non-negative")
	}
	if settings.CacheDefaults.SubscriptionTrafficTTLSeconds < 0 {
		return domain.NewError(domain.CodeInvalidArgument, "subscription_traffic_ttl_seconds must be non-negative")
	}
	return nil
}

func validateRuntimeHTTPURL(label, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return domain.WrapError(domain.CodeInvalidArgument, "parse "+label, err)
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return domain.NewError(domain.CodeInvalidArgument, label+" must use http or https")
	}
	if parsed.Host == "" {
		return domain.NewError(domain.CodeInvalidArgument, label+" host is required")
	}
	return nil
}

func validateRuntimeProxyURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return domain.WrapError(domain.CodeInvalidArgument, "parse remote proxy url", err)
	}
	switch parsed.Scheme {
	case "http", "https", "socks5":
	default:
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported remote proxy scheme %q", parsed.Scheme))
	}
	if parsed.Host == "" {
		return domain.NewError(domain.CodeInvalidArgument, "remote proxy host is required")
	}
	return nil
}
