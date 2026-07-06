package service

import (
	"fmt"
	"math"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) subscriptionTrafficFromRemote(sub domain.Subscription, remote *remoteInputResult) ([]domain.SubscriptionTrafficItem, []domain.Warning) {
	if remote == nil || subscriptionTrafficDisabled(sub.Remote) {
		return nil, nil
	}
	raw := strings.TrimSpace(remote.Headers.Get("Subscription-Userinfo"))
	if raw == "" {
		return nil, nil
	}
	item, warnings, hasUsage := s.parseSubscriptionTraffic(raw, sub.Name, sanitizedTrafficSourceURL(remote.SourceRef.URL))
	if !hasUsage {
		return nil, warnings
	}
	appURL := strings.TrimSpace(firstNonEmptyString(remote.Headers.Get("Profile-Web-Page-Url"), valueFromTrafficFields(raw, "app_url")))
	planName := strings.TrimSpace(firstNonEmptyString(remote.Headers.Get("Plan-Name"), remote.Headers.Get("Profile-Title"), valueFromTrafficFields(raw, "plan_name")))
	item.AppURL = firstNonEmptyString(item.AppURL, appURL)
	item.PlanName = firstNonEmptyString(item.PlanName, planName)
	if item.ObservedAt == "" {
		item.ObservedAt = s.now().UTC().Format(time.RFC3339)
	}
	if item.SourceName == "" {
		item.SourceName = sub.Name
	}
	if item.SourceURL == "" {
		item.SourceURL = sanitizedTrafficSourceURL(remote.SourceRef.URL)
	}
	return []domain.SubscriptionTrafficItem{item}, warnings
}

func (s *Service) parseSubscriptionTraffic(raw string, sourceName string, sourceURL string) (domain.SubscriptionTrafficItem, []domain.Warning, bool) {
	fields := parseTrafficFields(raw)
	item := domain.SubscriptionTrafficItem{
		SourceName: sourceName,
		SourceURL:  sourceURL,
		ObservedAt: s.now().UTC().Format(time.RFC3339),
	}
	warnings := []domain.Warning{}
	hasUsage := false
	invalidUsage := false
	warnInvalidField := func(key string) {
		warnings = append(warnings, domain.Warning{
			Code:    "subscription_traffic_parse_failed",
			Message: fmt.Sprintf("invalid subscription traffic field %q", key),
			Field:   key,
			Source:  sourceName,
		})
	}
	parseInt := func(key string) (int64, bool) {
		value, ok := fields[key]
		if !ok || strings.TrimSpace(value) == "" {
			return 0, false
		}
		parsed, ok := parseTrafficByteCount(value)
		if !ok {
			warnInvalidField(key)
			return 0, false
		}
		return parsed, true
	}
	if upload, ok := parseInt("upload"); ok {
		item.UploadBytes = upload
		hasUsage = true
	}
	if download, ok := parseInt("download"); ok {
		item.DownloadBytes = download
		hasUsage = true
	}
	if item.UploadBytes > math.MaxInt64-item.DownloadBytes {
		warnInvalidField("used_bytes")
		invalidUsage = true
	} else {
		item.UsedBytes = item.UploadBytes + item.DownloadBytes
	}
	if total, ok := parseInt("total"); ok {
		item.TotalBytes = &total
		remaining := total - item.UsedBytes
		item.RemainingBytes = &remaining
		hasUsage = true
	}
	if expire, ok := parseInt("expire"); ok && expire > 0 {
		expiresAt := time.Unix(expire, 0).UTC()
		item.ExpiresAt = expiresAt.Format(time.RFC3339)
		days := int(expiresAt.Sub(s.now().UTC()).Hours() / 24)
		item.RemainingDays = &days
	}
	if resetDay, ok := parseInt("reset_day"); ok {
		day := int(resetDay)
		item.ResetDay = &day
	}
	item.AppURL = strings.TrimSpace(fields["app_url"])
	item.PlanName = strings.TrimSpace(fields["plan_name"])
	return item, warnings, hasUsage && !invalidUsage
}

func parseTrafficByteCount(raw string) (int64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	rational, ok := new(big.Rat).SetString(value)
	if !ok || rational.Sign() < 0 {
		return 0, false
	}
	max := new(big.Rat).SetInt64(math.MaxInt64)
	if rational.Cmp(max) > 0 {
		return 0, false
	}
	return new(big.Int).Quo(rational.Num(), rational.Denom()).Int64(), true
}

func parseTrafficFields(raw string) map[string]string {
	fields := map[string]string{}
	for part := range strings.SplitSeq(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if _, exists := fields[key]; exists {
			continue
		}
		fields[key] = strings.TrimSpace(value)
	}
	return fields
}

func valueFromTrafficFields(raw string, key string) string {
	return parseTrafficFields(raw)[strings.ToLower(key)]
}

func subscriptionTrafficDisabled(remote *domain.RemoteInput) bool {
	if remote == nil {
		return false
	}
	parsed, err := url.Parse(remote.URL)
	if err != nil || parsed.Fragment == "" {
		return false
	}
	fragment, _ := url.QueryUnescape(parsed.Fragment)
	for _, part := range strings.FieldsFunc(fragment, func(r rune) bool { return r == '&' || r == ';' }) {
		key, value, ok := strings.Cut(part, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "noflow" {
			continue
		}
		if !ok || value == "" {
			return true
		}
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func sanitizedTrafficSourceURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.Fragment = ""
	return parsed.String()
}

func subscriptionTrafficItem(items []domain.SubscriptionTrafficItem) *domain.SubscriptionTrafficItem {
	cloned := cloneSubscriptionTrafficItems(items)
	if len(cloned) == 0 {
		return nil
	}
	return &cloned[0]
}

func cloneSubscriptionTrafficItem(item *domain.SubscriptionTrafficItem) *domain.SubscriptionTrafficItem {
	if item == nil {
		return nil
	}
	cloned := cloneSubscriptionTrafficItems([]domain.SubscriptionTrafficItem{*item})
	return &cloned[0]
}

func cloneSubscriptionTrafficItems(items []domain.SubscriptionTrafficItem) []domain.SubscriptionTrafficItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]domain.SubscriptionTrafficItem, len(items))
	for i, item := range items {
		out[i] = item
		if item.TotalBytes != nil {
			total := *item.TotalBytes
			out[i].TotalBytes = &total
		}
		if item.RemainingBytes != nil {
			remaining := *item.RemainingBytes
			out[i].RemainingBytes = &remaining
		}
		if item.RemainingDays != nil {
			remainingDays := *item.RemainingDays
			out[i].RemainingDays = &remainingDays
		}
		if item.ResetDay != nil {
			resetDay := *item.ResetDay
			out[i].ResetDay = &resetDay
		}
	}
	return out
}
