package service

import (
	"context"
	"sort"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/buildinfo"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type cacheBuild struct {
	Version  string `json:"version"`
	Revision string `json:"revision,omitempty"`
}

type cacheExecutionSettings struct {
	Remote domain.RemoteDefaults `json:"remote"`
	Probe  domain.ProbeDefaults  `json:"probe"`
	Script domain.ScriptDefaults `json:"script"`
}

type cacheDependencyRevision struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Revision string `json:"revision"`
}

func currentCacheBuild() cacheBuild {
	return cacheBuild{Version: buildinfo.Version(), Revision: buildinfo.Revision()}
}

func (s *Service) cacheExecutionSettings() cacheExecutionSettings {
	settings := s.currentSettings()
	return cacheExecutionSettings{
		Remote: settings.RemoteDefaults,
		Probe:  settings.ProbeDefaults,
		Script: settings.ScriptDefaults,
	}
}

func (s *Service) snapshotCacheDependencies(ctx context.Context, refs []domain.ResourceRef) ([]cacheDependencyRevision, error) {
	unique := map[string]domain.ResourceRef{}
	for _, ref := range refs {
		kind := strings.ToLower(strings.TrimSpace(ref.Kind))
		name := strings.TrimSpace(ref.Name)
		if name == "" || (kind != "subscription" && kind != "file") {
			continue
		}
		ref.Kind = kind
		ref.Name = name
		unique[kind+"\x00"+name] = ref
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]cacheDependencyRevision, 0, len(keys))
	for _, key := range keys {
		ref := unique[key]
		revision, err := s.cacheResourceRevision(ctx, ref)
		if err != nil {
			return nil, err
		}
		out = append(out, cacheDependencyRevision{Kind: ref.Kind, Name: ref.Name, Revision: revision})
	}
	return out, nil
}

func (s *Service) cacheDependenciesCurrent(ctx context.Context, dependencies []cacheDependencyRevision) bool {
	for _, dependency := range dependencies {
		revision, err := s.cacheResourceRevision(ctx, domain.ResourceRef{Kind: dependency.Kind, Name: dependency.Name})
		if err != nil || revision != dependency.Revision {
			return false
		}
	}
	return true
}

func (s *Service) cacheResourceRevision(ctx context.Context, ref domain.ResourceRef) (string, error) {
	if s.metaStore == nil {
		return "", storeUnavailable()
	}
	switch strings.ToLower(strings.TrimSpace(ref.Kind)) {
	case "subscription":
		subscription, err := s.metaStore.GetSubscription(ctx, strings.TrimSpace(ref.Name))
		if err != nil {
			return "", err
		}
		return cacheIdentity(subscription)
	case "file":
		file, err := s.metaStore.GetFile(ctx, strings.TrimSpace(ref.Name))
		if err != nil {
			return "", err
		}
		return cacheIdentity(file)
	default:
		return "", domain.NewError(domain.CodeInvalidArgument, "unsupported cache dependency kind")
	}
}
