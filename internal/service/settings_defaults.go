package service

import (
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) remoteInputWithDefaults(input domain.RemoteInput) domain.RemoteInput {
	settings := s.currentSettings()
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
	return out
}

func (s *Service) probeRequestWithDefaults(req domain.ProbeRequest) domain.ProbeRequest {
	settings := s.currentSettings()
	defaults := settings.ProbeDefaults
	out := req
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
	return out
}
