package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

//go:embed catalog_builtin
var builtInRuleSetCatalogFiles embed.FS

const builtInRuleSetCatalogPath = "catalog_builtin/catalog.json.gz"

var ErrRuleSetCatalogUnavailable = errors.New("rule-set catalog is unavailable")

var (
	builtInRuleSetCatalogOnce sync.Once
	builtInRuleSetCatalog     *ruleSetCatalogSnapshot
	builtInRuleSetCatalogErr  error
)

type RuleSetCatalogItem struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	RuleKind      string `json:"rule_kind"`
	ReferenceType string `json:"reference_type,omitempty"`
}

type RuleSetCatalogResult struct {
	Items []RuleSetCatalogItem `json:"items"`
}

type ruleSetCatalogSnapshot struct {
	Mihomo       []RuleSetCatalogItem `json:"mihomo"`
	SingBox      []RuleSetCatalogItem `json:"sing-box"`
	Shadowrocket []RuleSetCatalogItem `json:"shadowrocket"`
}

// WithRuleSetCatalogSnapshot replaces the embedded snapshot in contract tests.
func WithRuleSetCatalogSnapshot(body []byte) Option {
	catalog, err := loadRuleSetCatalog(body)
	return func(s *Service) {
		s.catalog = func() (*ruleSetCatalogSnapshot, error) { return catalog, err }
	}
}

func (s *Service) ListRuleSetCatalog(ctx context.Context, target string) (*RuleSetCatalogResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target = strings.TrimSpace(target)
	if target != "mihomo" && target != "sing-box" && target != "shadowrocket" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "rule-set catalog target must be mihomo, sing-box, or shadowrocket")
	}
	loader := s.catalog
	if loader == nil {
		loader = loadEmbeddedRuleSetCatalog
	}
	catalog, err := loader()
	if err != nil {
		return nil, err
	}
	items := catalog.Mihomo
	switch target {
	case "sing-box":
		items = catalog.SingBox
	case "shadowrocket":
		items = catalog.Shadowrocket
	}
	return &RuleSetCatalogResult{Items: append([]RuleSetCatalogItem(nil), items...)}, nil
}

func loadEmbeddedRuleSetCatalog() (*ruleSetCatalogSnapshot, error) {
	builtInRuleSetCatalogOnce.Do(func() {
		body, err := builtInRuleSetCatalogFiles.ReadFile(builtInRuleSetCatalogPath)
		if errors.Is(err, fs.ErrNotExist) {
			builtInRuleSetCatalogErr = unavailableCatalogError("snapshot is not included", nil)
			return
		}
		if err != nil {
			builtInRuleSetCatalogErr = unavailableCatalogError("read snapshot", err)
			return
		}
		builtInRuleSetCatalog, builtInRuleSetCatalogErr = loadRuleSetCatalog(body)
	})
	return builtInRuleSetCatalog, builtInRuleSetCatalogErr
}

func loadRuleSetCatalog(body []byte) (*ruleSetCatalogSnapshot, error) {
	compressed, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, unavailableCatalogError("open snapshot", err)
	}
	decoded, readErr := io.ReadAll(compressed)
	closeErr := compressed.Close()
	if readErr != nil {
		return nil, unavailableCatalogError("decompress snapshot", readErr)
	}
	if closeErr != nil {
		return nil, unavailableCatalogError("decompress snapshot", closeErr)
	}
	var catalog ruleSetCatalogSnapshot
	if err := json.Unmarshal(decoded, &catalog); err != nil {
		return nil, unavailableCatalogError("decode snapshot", err)
	}
	if !validCatalogItems("mihomo", catalog.Mihomo) ||
		!validCatalogItems("sing-box", catalog.SingBox) ||
		!validCatalogItems("shadowrocket", catalog.Shadowrocket) {
		return nil, unavailableCatalogError("snapshot contains invalid items", nil)
	}
	return &catalog, nil
}

func validCatalogItems(target string, items []RuleSetCatalogItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Name == "" || item.URL == "" {
			return false
		}
		if target != "shadowrocket" {
			if item.ReferenceType != "" || (item.RuleKind != "domain" && item.RuleKind != "ip") {
				return false
			}
			continue
		}
		switch item.ReferenceType {
		case "DOMAIN-SET":
			if item.RuleKind != "domain" {
				return false
			}
		case "RULE-SET":
			if item.RuleKind != "mixed" && item.RuleKind != "ip" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func unavailableCatalogError(message string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", ErrRuleSetCatalogUnavailable, message)
	}
	return fmt.Errorf("%w: %s: %w", ErrRuleSetCatalogUnavailable, message, cause)
}
