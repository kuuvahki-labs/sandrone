package store

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type MetaStore struct {
	store     Store
	summaries summaryCache
}

func NewMetaStore(store Store) *MetaStore {
	return &MetaStore{store: store, summaries: summaryCache{now: time.Now}}
}

func (s *MetaStore) PutSubscription(ctx context.Context, sub domain.Subscription) error {
	if sub.Name == "" {
		return fmt.Errorf("%w: subscription name is required", ErrInvalidKey)
	}
	sub.DisplayName = strings.TrimSpace(sub.DisplayName)
	return s.writeJSON(ctx, "subscriptions", sub.Name, sub)
}

func (s *MetaStore) GetSubscription(ctx context.Context, name string) (domain.Subscription, error) {
	return s.readJSON[domain.Subscription](ctx, "subscriptions", name)
}

func (s *MetaStore) ListSubscriptions(ctx context.Context) ([]domain.ResourceSummary, error) {
	return s.list(ctx, "subscription", "subscriptions", func(body []byte, summary *domain.ResourceSummary) {
		var sub domain.Subscription
		if err := json.Unmarshal(body, &sub); err != nil {
			summary.Warning = err.Error()
			return
		}
		summary.Type = string(sub.Type)
		summary.DisplayName = sub.DisplayName
		summary.Format = sub.Format
		summary.Meta = sub.Meta
	})
}

func (s *MetaStore) PutFile(ctx context.Context, file domain.FileSpec) error {
	if file.Name == "" {
		return fmt.Errorf("%w: file name is required", ErrInvalidKey)
	}
	file = normalizeFileForStorage(file)
	return s.writeJSON(ctx, "files", file.Name, file)
}

func (s *MetaStore) GetFile(ctx context.Context, name string) (domain.FileSpec, error) {
	return s.readJSON[domain.FileSpec](ctx, "files", name)
}

func (s *MetaStore) ListFiles(ctx context.Context) ([]domain.ResourceSummary, error) {
	return s.list(ctx, "file", "files", func(body []byte, summary *domain.ResourceSummary) {
		var file domain.FileSpec
		if err := json.Unmarshal(body, &file); err != nil {
			summary.Warning = err.Error()
			return
		}
		summary.Type = file.Source.Type
		summary.Target = string(file.Kind)
		summary.DisplayName = file.DisplayName
		summary.Meta = file.Meta
		summary.Processors = file.Processors
	})
}

func (s *MetaStore) PutShare(ctx context.Context, share domain.Share) error {
	if share.ID == "" {
		return fmt.Errorf("%w: share id is required", ErrInvalidKey)
	}
	return s.writeJSON(ctx, "shares", share.ID, share)
}

func (s *MetaStore) GetShare(ctx context.Context, id string) (domain.Share, error) {
	return s.readJSON[domain.Share](ctx, "shares", id)
}

func (s *MetaStore) ListShares(ctx context.Context) ([]domain.Share, error) {
	entries, generation, err := s.listEntries(ctx, "shares")
	if err != nil {
		return nil, err
	}
	out, errs := mapMetadata(ctx, s.store, entries, func(entry ListedEntry) (domain.Share, error) {
		return s.readSummary(ctx, entry, generation, func(body []byte) (domain.Share, error) {
			var share domain.Share
			err := json.Unmarshal(body, &share)
			return share, err
		})
	})
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *MetaStore) DeleteSubscription(ctx context.Context, name string) error {
	return s.deleteResource(ctx, "subscriptions", name)
}

func (s *MetaStore) DeleteFile(ctx context.Context, name string) error {
	return s.deleteResource(ctx, "files", name)
}

func (s *MetaStore) DeleteShare(ctx context.Context, id string) error {
	return s.deleteResource(ctx, "shares", id)
}

func (s *MetaStore) writeJSON(ctx context.Context, prefix string, name string, value any) error {
	key, err := resourceKey(prefix, name)
	if err != nil {
		return err
	}
	body, err := marshalStoreJSON(value)
	if err != nil {
		return err
	}
	defer s.summaries.invalidate(key)
	return s.store.Write(ctx, key, body)
}

func marshalStoreJSON(value any) ([]byte, error) {
	return json.Marshal(value, json.Deterministic(true), jsontext.WithIndent("  "))
}

func (s *MetaStore) readJSON[T any](ctx context.Context, prefix string, name string) (T, error) {
	var out T
	key, err := resourceKey(prefix, name)
	if err != nil {
		return out, err
	}
	body, err := s.store.Read(ctx, key)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (s *MetaStore) deleteResource(ctx context.Context, prefix string, name string) error {
	key, err := resourceKey(prefix, name)
	if err != nil {
		return err
	}
	defer s.summaries.invalidate(key)
	return s.store.Delete(ctx, key)
}

func (s *MetaStore) list(ctx context.Context, kind, prefix string, enrich func([]byte, *domain.ResourceSummary)) ([]domain.ResourceSummary, error) {
	entries, generation, err := s.listEntries(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out, _ := mapMetadata(ctx, s.store, entries, func(entry ListedEntry) (domain.ResourceSummary, error) {
		name := strings.TrimSuffix(strings.TrimPrefix(entry.Key, prefix+"/"), ".json")
		summary := domain.ResourceSummary{Kind: kind, Name: name, Size: entry.Size}
		parsed, err := s.readSummary(ctx, entry, generation, func(body []byte) (domain.ResourceSummary, error) {
			populateResourceSummaryTimestamps(body, &summary)
			if enrich != nil {
				enrich(body, &summary)
			}
			if summary.Warning != "" {
				return summary, fmt.Errorf("%s", summary.Warning)
			}
			return summary, nil
		})
		if err != nil {
			summary.Warning = err.Error()
		} else {
			summary = parsed
			summary.Size = entry.Size
		}

		return summary, nil
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func populateResourceSummaryTimestamps(body []byte, summary *domain.ResourceSummary) {
	var timestamps struct {
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &timestamps); err != nil {
		return
	}
	summary.CreatedAt = timestamps.CreatedAt
	summary.UpdatedAt = timestamps.UpdatedAt
}

func resourceKey(prefix string, name string) (string, error) {
	name, err := CleanKey(name)
	if err != nil {
		return "", err
	}
	return prefix + "/" + name + ".json", nil
}

func normalizeFileForStorage(file domain.FileSpec) domain.FileSpec {
	file.DisplayName = strings.TrimSpace(file.DisplayName)
	file.Source.Type = strings.ToLower(strings.TrimSpace(file.Source.Type))
	switch file.Source.Type {
	case "inline":
		file.Source.Remote = nil
	case "remote":
		file.Source.Content = ""
	}
	return file
}
