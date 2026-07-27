package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/metacubex/age"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) PutSubscription(ctx context.Context, sub domain.Subscription) error {
	if s.metaStore == nil {
		return storeUnavailable()
	}
	normalized, err := normalizeSubscription(sub)
	if err != nil {
		return err
	}
	if err := s.metaStore.PutSubscription(ctx, normalized); err != nil {
		return err
	}
	s.invalidateSubscriptionTrafficCache(ctx)
	s.invalidateResultCaches(ctx)
	s.logResource(ctx, "put", "subscription", normalized.Name)
	return nil
}

func (s *Service) PutFile(ctx context.Context, file domain.FileSpec) error {
	if err := s.validateFileSpecStructure(file); err != nil {
		return err
	}
	if s.metaStore == nil {
		return storeUnavailable()
	}
	if err := s.metaStore.PutFile(ctx, file); err != nil {
		return err
	}
	s.invalidateResultCaches(ctx)
	s.logResource(ctx, "put", "file", file.Name)
	return nil
}

func (s *Service) DeleteSubscription(ctx context.Context, name string) error {
	if s.metaStore == nil {
		return storeUnavailable()
	}
	if err := s.metaStore.DeleteSubscription(ctx, name); err != nil {
		return err
	}
	s.invalidateSubscriptionTrafficCache(ctx)
	s.invalidateResultCaches(ctx)
	s.logResource(ctx, "delete", "subscription", name)
	return nil
}

func (s *Service) DeleteFile(ctx context.Context, name string) error {
	if s.metaStore == nil {
		return storeUnavailable()
	}
	if err := s.metaStore.DeleteFile(ctx, name); err != nil {
		return err
	}
	s.invalidateResultCaches(ctx)
	s.logResource(ctx, "delete", "file", name)
	return nil
}

func (s *Service) GetSubscription(ctx context.Context, name string) (*domain.Subscription, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	sub, err := s.metaStore.GetSubscription(ctx, name)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Service) ListSubscriptions(ctx context.Context) (*domain.ResourceListResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	items, err := s.metaStore.ListSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	return &domain.ResourceListResult{Items: items}, nil
}

func (s *Service) GetFileSpec(ctx context.Context, name string) (*domain.FileSpec, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	file, err := s.metaStore.GetFile(ctx, name)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// GetFileSource resolves the stored file input before typed configuration
// compilation and file-stage processors are applied.
func (s *Service) GetFileSource(ctx context.Context, name string) (*domain.FileDocument, error) {
	spec, err := s.GetFileSpec(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := s.validateFileSpecStructure(*spec); err != nil {
		return nil, err
	}
	kind := spec.Kind
	var doc domain.FileDocument
	switch kind {
	case domain.FileKindStatic:
		doc, _, err = s.resolveFileSource(ctx, *spec)
		doc.Kind = string(domain.FileKindStatic)
		doc.MediaType = "text/plain; charset=utf-8"
	default:
		driver, lookupErr := s.typedFiles.Lookup(kind)
		if lookupErr != nil {
			return nil, lookupErr
		}
		descriptor := driver.Descriptor()
		if strings.TrimSpace(spec.Source.Type) == "" {
			doc = domain.FileDocument{
				Name: spec.Name, Kind: string(kind), MediaType: descriptor.MediaType,
				Content: append([]byte(nil), descriptor.DefaultBase...), Meta: cloneStringMap(spec.Meta),
			}
		} else {
			doc, _, err = s.resolveFileSource(ctx, *spec)
			doc.Kind = string(kind)
			doc.MediaType = descriptor.MediaType
		}
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *Service) ListFiles(ctx context.Context) (*domain.ResourceListResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	items, err := s.metaStore.ListFiles(ctx)
	if err != nil {
		return nil, err
	}
	return &domain.ResourceListResult{Items: items}, nil
}

func (s *Service) CreateShare(ctx context.Context, req domain.ShareCreateRequest) (*domain.ShareCreateResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	if err := validateShareTarget(req.TargetKind, req.TargetName); err != nil {
		return nil, err
	}
	if err := validateShareTimeRange(req.ValidFrom, req.ValidUntil); err != nil {
		return nil, err
	}
	if req.MaxUses < 0 {
		return nil, domain.NewError(domain.CodeInvalidArgument, "share max_uses must not be negative")
	}
	ageRecipient := strings.TrimSpace(req.AgeRecipient)
	if ageRecipient != "" {
		if _, err := age.ParseX25519Recipient(ageRecipient); err != nil {
			return nil, domain.WrapError(domain.CodeInvalidArgument, "share age_recipient must be one X25519 public key", err)
		}
	}
	if err := s.ensureShareTargetExists(ctx, req.TargetKind, req.TargetName); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = newShareID()
	}
	kind := strings.ToLower(strings.TrimSpace(req.TargetKind))
	targetFormat := strings.TrimSpace(req.TargetFormat)
	if kind == "subscription" && targetFormat == "" {
		targetFormat = "base64"
	}
	now := s.now().UTC()
	share := domain.Share{
		ID:           id,
		Name:         strings.TrimSpace(req.Name),
		TargetKind:   kind,
		TargetName:   strings.TrimSpace(req.TargetName),
		TargetFormat: targetFormat,
		ContentType:  strings.TrimSpace(req.ContentType),
		CreatedAt:    now,
		UpdatedAt:    now,
		ValidFrom:    req.ValidFrom.UTC(),
		ValidUntil:   req.ValidUntil.UTC(),
		AgeRecipient: ageRecipient,
		MaxUses:      req.MaxUses,
		Meta:         cloneStringMap(req.Meta),
	}
	if err := s.metaStore.CreateShare(ctx, share); err != nil {
		return nil, err
	}
	s.logResource(ctx, "put", "share", share.ID)
	return &domain.ShareCreateResult{
		Share:        share,
		Presentation: sharePresentation(share),
	}, nil
}

func (s *Service) ensureShareTargetExists(ctx context.Context, kind string, name string) error {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "file":
		_, err := s.metaStore.GetFile(ctx, strings.TrimSpace(name))
		return err
	case "subscription":
		_, err := s.metaStore.GetSubscription(ctx, strings.TrimSpace(name))
		return err
	default:
		return domain.NewError(domain.CodeInvalidArgument, "unsupported share target kind")
	}
}

func (s *Service) ListShares(ctx context.Context) (*domain.ShareListResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	shares, err := s.metaStore.ListShares(ctx)
	if err != nil {
		return nil, err
	}
	presentations := make(map[string]domain.SharePresentation, len(shares))
	for _, share := range shares {
		presentations[share.ID] = sharePresentation(share)
	}
	return &domain.ShareListResult{Shares: shares, Presentations: presentations}, nil
}

func (s *Service) GetShare(ctx context.Context, id string) (*domain.Share, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	share, err := s.metaStore.GetShare(ctx, id)
	if err != nil {
		return nil, err
	}
	return &share, nil
}

func (s *Service) DeleteShare(ctx context.Context, id string) error {
	if s.metaStore == nil {
		return storeUnavailable()
	}
	if err := s.metaStore.DeleteShare(ctx, id); err != nil {
		return err
	}
	s.logResource(ctx, "delete", "share", id)
	return nil
}

func (s *Service) logResource(ctx context.Context, action string, resourceType string, resourceName string) {
	message := "service resource updated"
	if action == "delete" {
		message = "service resource deleted"
	}
	s.log(ctx, slog.LevelInfo, message,
		"operation", action+"_resource",
		"resource_type", resourceType,
		"resource_name", strings.TrimSpace(resourceName),
	)
}

func (s *Service) RenderShare(ctx context.Context, req domain.ShareRenderRequest) (*domain.ShareRenderResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	share, err := s.metaStore.GetShare(ctx, strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}
	if !shareCurrentlyValid(share, s.now()) {
		return nil, os.ErrNotExist
	}
	var out domain.ShareRenderResult
	var finalFileName string
	var format string
	switch strings.ToLower(strings.TrimSpace(share.TargetKind)) {
	case "file":
		result, err := s.GetFile(ctx, domain.FileRequest{Name: share.TargetName, Request: req.Request})
		if err != nil {
			return nil, err
		}
		out.ContentType = firstNonEmptyString(share.ContentType, result.Response.Headers["Content-Type"], result.ContentType, "application/octet-stream")
		out.Body = append([]byte{}, result.Content...)
		out.Headers = cloneStringMap(result.Response.Headers)
		out.Status = result.Response.Status
		finalFileName = result.File.Name
	case "subscription":
		format = strings.TrimSpace(req.Format)
		if format == "" {
			format = strings.TrimSpace(share.TargetFormat)
		}
		if format == "" {
			format = "base64"
		}
		result, err := s.RenderSubscription(ctx, share.TargetName, format, req.Request)
		if err != nil {
			return nil, err
		}
		out.ContentType = firstNonEmptyString(share.ContentType, result.ContentType, "application/octet-stream")
		out.Body = append([]byte{}, result.Body...)
	default:
		return nil, domain.NewError(domain.CodeInvalidArgument, "unsupported share target kind")
	}
	if share.AgeRecipient != "" {
		if err := encryptShareResult(&out, share.AgeRecipient); err != nil {
			return nil, err
		}
	}
	filename := shareResponseFilename(share, finalFileName, format)
	if req.PresentedFilename != "" && req.PresentedFilename != filename {
		return nil, domain.NewError(domain.CodeInvalidArgument, "share filename does not match the requested format")
	}
	setShareContentDisposition(&out, filename)
	if _, err := s.metaStore.ConsumeShare(ctx, share.ID, s.now()); err != nil {
		return nil, err
	}
	return &out, nil
}

func encryptShareResult(result *domain.ShareRenderResult, recipientText string) error {
	recipient, err := age.ParseX25519Recipient(recipientText)
	if err != nil {
		return domain.WrapError(domain.CodeInvalidArgument, "parse share age recipient", err)
	}
	originalContentType := result.ContentType
	var encrypted bytes.Buffer
	writer, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		return domain.WrapError(domain.CodeRenderFailed, "initialize share age encryption", err)
	}
	if _, err := writer.Write(result.Body); err != nil {
		return domain.WrapError(domain.CodeRenderFailed, "encrypt shared content", err)
	}
	if err := writer.Close(); err != nil {
		return domain.WrapError(domain.CodeRenderFailed, "finalize share age encryption", err)
	}
	result.Body = encrypted.Bytes()
	result.ContentType = "application/age"
	if result.Headers == nil {
		result.Headers = map[string]string{}
	}
	result.Headers["X-Sandrone-Original-Content-Type"] = originalContentType
	result.Headers["Content-Type"] = "application/age"
	return nil
}

func (s *Service) Inspect(ctx context.Context, req domain.InspectRequest) (*domain.InspectResult, error) {
	capabilities := s.CapabilitySummary()
	if s.metaStore != nil {
		if subscriptions, err := s.metaStore.ListSubscriptions(ctx); err == nil {
			capabilities["subscriptions"] = len(subscriptions)
		}
		if files, err := s.metaStore.ListFiles(ctx); err == nil {
			capabilities["files"] = len(files)
		}
		capabilities["store_configured"] = true
	} else {
		capabilities["store_configured"] = false
	}
	report := s.prepareReport("inspect", domain.Report{})
	return &domain.InspectResult{Capabilities: capabilities, Report: report}, nil
}
func (s *Service) prepareReport(kind string, report domain.Report) domain.Report {
	if report.Kind == "" {
		report.Kind = kind
	}
	if report.Status == "" {
		report.Status = "ok"
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = s.now().UTC()
	}
	if len(report.Refs) == 0 && len(report.Dependencies) > 0 {
		report.Refs = append([]domain.ResourceRef{}, report.Dependencies...)
	}
	for _, warning := range report.Warnings {
		if warning.Code == "lossy" || strings.Contains(warning.Code, "loss") || strings.Contains(warning.Code, "unsupported") {
			report.Lossy = true
			break
		}
	}
	if report.Render.LostFields > 0 {
		report.Lossy = true
	}
	return report
}
func validateShareTarget(kind, name string) error {
	kind = strings.ToLower(strings.TrimSpace(kind))
	name = strings.TrimSpace(name)
	switch kind {
	case "file", "subscription":
	default:
		return domain.NewError(domain.CodeInvalidArgument, "share target_kind must be file or subscription")
	}
	if name == "" {
		return domain.NewError(domain.CodeInvalidArgument, "share target_name is required")
	}
	return nil
}

func validateShareTimeRange(from, until time.Time) error {
	if !from.IsZero() && !until.IsZero() && !from.Before(until) {
		return domain.NewError(domain.CodeInvalidArgument, "share valid_from must be before valid_until")
	}
	return nil
}

func shareCurrentlyValid(share domain.Share, now time.Time) bool {
	now = now.UTC()
	if !share.ValidFrom.IsZero() && now.Before(share.ValidFrom.UTC()) {
		return false
	}
	if !share.ValidUntil.IsZero() && !now.Before(share.ValidUntil.UTC()) {
		return false
	}
	if share.MaxUses > 0 && share.UseCount >= share.MaxUses {
		return false
	}
	return true
}

func newShareID() string {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Sprintf("sh_%d", time.Now().UnixNano())
	}
	return "sh_" + hex.EncodeToString(suffix[:])
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
