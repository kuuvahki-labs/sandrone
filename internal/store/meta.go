package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type MetaStore struct {
	store Store
}

func NewMetaStore(store Store) *MetaStore {
	return &MetaStore{store: store}
}

func (s *MetaStore) PutSubscription(ctx context.Context, sub domain.Subscription) error {
	if sub.Name == "" {
		return fmt.Errorf("%w: subscription name is required", ErrInvalidKey)
	}
	sub.DisplayName = strings.TrimSpace(sub.DisplayName)
	return s.writeJSON(ctx, "subscriptions", sub.Name, sub)
}

func (s *MetaStore) GetSubscription(ctx context.Context, name string) (domain.Subscription, error) {
	var sub domain.Subscription
	err := s.readJSON(ctx, "subscriptions", name, &sub)
	return sub, err
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
	var file domain.FileSpec
	err := s.readJSON(ctx, "files", name, &file)
	return file, err
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
	key, err := resourceKey("shares", share.ID)
	if err != nil {
		return err
	}
	body, err := marshalStoreJSON(share)
	if err != nil {
		return err
	}
	return s.store.Write(ctx, key, body)
}

func (s *MetaStore) GetShare(ctx context.Context, id string) (domain.Share, error) {
	key, err := resourceKey("shares", id)
	if err != nil {
		return domain.Share{}, err
	}
	body, err := s.store.Read(ctx, key)
	if err != nil {
		return domain.Share{}, err
	}
	var share domain.Share
	if err := json.Unmarshal(body, &share); err != nil {
		return domain.Share{}, err
	}
	return share, nil
}

func (s *MetaStore) ListShares(ctx context.Context) ([]domain.Share, error) {
	entries, err := s.store.List(ctx, "shares")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domain.Share{}, nil
		}
		return nil, err
	}
	out := make([]domain.Share, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir || !strings.HasSuffix(entry.Key, ".json") {
			continue
		}
		body, err := s.store.Read(ctx, entry.Key)
		if err != nil {
			return nil, err
		}
		var share domain.Share
		if err := json.Unmarshal(body, &share); err != nil {
			return nil, err
		}
		out = append(out, share)
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
	return writeJSON(ctx, s.store, prefix, name, value)
}

func writeJSON(ctx context.Context, resourceStore Store, prefix string, name string, value any) error {
	key, err := resourceKey(prefix, name)
	if err != nil {
		return err
	}
	body, err := marshalStoreJSON(value)
	if err != nil {
		return err
	}
	return resourceStore.Write(ctx, key, body)
}

func marshalStoreJSON(value any) ([]byte, error) {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func (s *MetaStore) readJSON(ctx context.Context, prefix string, name string, out any) error {
	return readJSON(ctx, s.store, prefix, name, out)
}

func readJSON(ctx context.Context, resourceStore Store, prefix string, name string, out any) error {
	key, err := resourceKey(prefix, name)
	if err != nil {
		return err
	}
	body, err := resourceStore.Read(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (s *MetaStore) deleteResource(ctx context.Context, prefix string, name string) error {
	return deleteResource(ctx, s.store, prefix, name)
}

func deleteResource(ctx context.Context, resourceStore Store, prefix string, name string) error {
	key, err := resourceKey(prefix, name)
	if err != nil {
		return err
	}
	return resourceStore.Delete(ctx, key)
}

func (s *MetaStore) list(ctx context.Context, kind, prefix string, enrich func([]byte, *domain.ResourceSummary)) ([]domain.ResourceSummary, error) {
	entries, err := s.store.List(ctx, prefix)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domain.ResourceSummary{}, nil
		}
		return nil, err
	}
	out := make([]domain.ResourceSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir || !strings.HasSuffix(entry.Key, ".json") {
			continue
		}
		name := strings.TrimPrefix(entry.Key, prefix+"/")
		name = strings.TrimSuffix(name, ".json")
		summary := domain.ResourceSummary{
			Kind: kind,
			Name: name,
			Size: entry.Size,
		}
		if enrich != nil {
			body, err := s.store.Read(ctx, entry.Key)
			if err != nil {
				summary.Warning = err.Error()
			} else {
				populateResourceSummaryTimestamps(body, &summary)
				enrich(body, &summary)
			}
		}
		out = append(out, summary)
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
