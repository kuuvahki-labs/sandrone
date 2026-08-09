package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type HysteriaImplicitUnit string

const (
	HysteriaImplicitNone HysteriaImplicitUnit = ""
	HysteriaImplicitMbps HysteriaImplicitUnit = "Mbps"
	HysteriaImplicitBps  HysteriaImplicitUnit = "Bps"
)

type HysteriaRate struct {
	Text string
	Mbps int
}

func MaxHysteriaMbps() int {
	max, _ := checkedHysteriaMbpsInt(maxHysteriaMbpsUint64())
	return max
}

func maxHysteriaMbpsUint64() uint64 {
	max := uint64(math.MaxUint64 / 1_000_000)
	if archMax := maxUint64ForArch(); archMax < max {
		max = archMax
	}
	return max
}

func checkedHysteriaMbpsInt(value uint64) (int, bool) {
	if value > maxHysteriaMbpsUint64() {
		return 0, false
	}
	return int(value), true //nolint:gosec // The architecture-specific upper bound is checked above.
}

func NormalizeHysteriaMbps(value any) (int, error) {
	var parsed uint64
	switch typed := value.(type) {
	case int:
		if typed < 0 {
			return 0, fmt.Errorf("hysteria Mbps rate must not be negative")
		}
		parsed = uint64(typed)
	case int64:
		if typed < 0 {
			return 0, fmt.Errorf("hysteria Mbps rate must not be negative")
		}
		parsed = uint64(typed)
	case uint64:
		parsed = typed
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed < 0 || math.Trunc(typed) != typed {
			return 0, fmt.Errorf("hysteria Mbps rate must be a non-negative integer")
		}
		if typed > float64(MaxHysteriaMbps()) {
			return 0, fmt.Errorf("hysteria Mbps rate exceeds safe bound")
		}
		parsed = uint64(typed)
	case json.Number:
		var err error
		parsed, err = strconv.ParseUint(string(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid integer Hysteria Mbps rate: %w", err)
		}
	case string:
		var err error
		parsed, err = strconv.ParseUint(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid integer Hysteria Mbps rate: %w", err)
		}
	default:
		return 0, fmt.Errorf("unsupported Hysteria Mbps type %T", value)
	}
	result, ok := checkedHysteriaMbpsInt(parsed)
	if !ok {
		return 0, fmt.Errorf("hysteria Mbps rate exceeds safe bound")
	}
	return result, nil
}

var hysteriaRateBitMultipliers = map[string]uint64{
	"bps": 1, "Bps": 8,
	"Kbps": 1_000, "KBps": 8_000,
	"Mbps": 1_000_000, "MBps": 8_000_000,
	"Gbps": 1_000_000_000, "GBps": 8_000_000_000,
	"Tbps": 1_000_000_000_000, "TBps": 8_000_000_000_000,
}

func NormalizeHysteriaRate(raw string, implicit HysteriaImplicitUnit) (HysteriaRate, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return HysteriaRate{}, fmt.Errorf("hysteria rate is empty")
	}

	digitsEnd := 0
	for digitsEnd < len(raw) && raw[digitsEnd] >= '0' && raw[digitsEnd] <= '9' {
		digitsEnd++
	}
	if digitsEnd == 0 {
		return HysteriaRate{}, fmt.Errorf("invalid hysteria rate %q", raw)
	}

	unit := strings.TrimSpace(raw[digitsEnd:])
	if unit == "" {
		unit = string(implicit)
	}
	bitMultiplier, ok := hysteriaRateBitMultipliers[unit]
	if !ok {
		return HysteriaRate{}, fmt.Errorf("invalid hysteria rate unit %q", unit)
	}

	value, err := strconv.ParseUint(raw[:digitsEnd], 10, 64)
	if err != nil {
		return HysteriaRate{}, fmt.Errorf("invalid hysteria rate value: %w", err)
	}
	if value == 0 {
		return HysteriaRate{}, fmt.Errorf("hysteria rate must be positive")
	}
	if bitMultiplier > math.MaxUint64/value {
		return HysteriaRate{}, fmt.Errorf("hysteria rate overflows uint64")
	}

	bits := value * bitMultiplier
	if bits%1_000_000 == 0 {
		mbps := bits / 1_000_000
		if parsed, ok := checkedHysteriaMbpsInt(mbps); ok {
			return HysteriaRate{Mbps: parsed}, nil
		}
	}
	return HysteriaRate{Text: strconv.FormatUint(value, 10) + " " + unit}, nil
}

func ExactHysteriaMbps(text string) (int, bool) {
	rate, err := NormalizeHysteriaRate(text, HysteriaImplicitNone)
	if err != nil || rate.Mbps <= 0 {
		return 0, false
	}
	return rate.Mbps, true
}

func ValidateCanonicalHysteriaBandwidth(options *domain.HysteriaOptions) error {
	if options == nil {
		return fmt.Errorf("hysteria options are required")
	}
	if err := validateCanonicalHysteriaRate("up", options.Up, options.UpMbps); err != nil {
		return err
	}
	return validateCanonicalHysteriaRate("down", options.Down, options.DownMbps)
}

func NormalizeLegacyHysteriaBandwidth(node *domain.NodeIR) []domain.Warning {
	if node == nil || node.Type != domain.NodeTypeHysteria || node.Hysteria == nil {
		return nil
	}

	implicit, textFirst, warnImplicit := legacyHysteriaSourcePolicy(node.SourceFormat)
	warnings := normalizeLegacyHysteriaDirection(node, "up", &node.Hysteria.Up, &node.Hysteria.UpMbps, implicit, textFirst, warnImplicit)
	warnings = append(warnings, normalizeLegacyHysteriaDirection(node, "down", &node.Hysteria.Down, &node.Hysteria.DownMbps, implicit, textFirst, warnImplicit)...)
	return warnings
}

func NormalizeURIHysteriaBandwidth(node *domain.NodeIR, source *domain.SourceInfo, values url.Values) map[string]bool {
	known := map[string]bool{}
	known["up"], known["upmbps"] = normalizeURIHysteriaRate(node, source, values, "up", &node.Hysteria.Up, &node.Hysteria.UpMbps)
	known["down"], known["downmbps"] = normalizeURIHysteriaRate(node, source, values, "down", &node.Hysteria.Down, &node.Hysteria.DownMbps)
	return known
}

func normalizeURIHysteriaRate(node *domain.NodeIR, source *domain.SourceInfo, values url.Values, direction string, text *string, mbps *int) (bool, bool) {
	compatKey := direction + "mbps"
	_, hasText := values[direction]
	compatKnown := false
	if _, hasCompat := values[compatKey]; hasCompat {
		compat, err := NormalizeHysteriaMbps(values.Get(compatKey))
		if err == nil {
			compatKnown = true
			if compat > 0 {
				*mbps = compat
				if !hasText {
					return false, true
				}
				_, err := NormalizeHysteriaRate(strings.TrimSpace(values.Get(direction)), HysteriaImplicitMbps)
				return err == nil, true
			}
		}
	}
	if !hasText {
		return false, compatKnown
	}

	raw := strings.TrimSpace(values.Get(direction))
	rate, err := NormalizeHysteriaRate(raw, HysteriaImplicitMbps)
	if err != nil {
		return false, compatKnown
	}
	*text, *mbps = rate.Text, rate.Mbps
	if isBareHysteriaRate(raw) {
		source.Warnings = append(source.Warnings, domain.Warning{
			Code:    "parse_implicit_bandwidth_unit",
			Message: "bare Hysteria bandwidth assumed to be Mbps",
			Node:    node.Name,
			Field:   "hysteria." + direction,
			Source:  "uri",
		})
	}
	return true, compatKnown
}

func legacyHysteriaSourcePolicy(source string) (HysteriaImplicitUnit, bool, bool) {
	switch canonicalSourceFormat(source) {
	case "sing-box":
		return HysteriaImplicitBps, true, false
	case "mihomo", "uri-list":
		return HysteriaImplicitMbps, false, false
	case "json-nodes", "":
		return HysteriaImplicitMbps, false, true
	default:
		return HysteriaImplicitMbps, false, true
	}
}

func normalizeLegacyHysteriaDirection(
	node *domain.NodeIR,
	direction string,
	text *string,
	mbps *int,
	implicit HysteriaImplicitUnit,
	textFirst bool,
	warnImplicit bool,
) []domain.Warning {
	rawText := strings.TrimSpace(*text)
	warnings := []domain.Warning{}
	if textFirst && rawText != "" {
		shadowedMbps := *mbps
		*mbps = 0
		warnings = append(warnings, applyLegacyHysteriaText(node, direction, rawText, text, mbps, implicit, warnImplicit)...)
		if _, err := NormalizeHysteriaMbps(shadowedMbps); err != nil {
			warnings = append(warnings, preserveLegacyHysteriaValue(node, direction, direction+"_mbps", shadowedMbps)...)
		}
		return warnings
	}
	parsedMbps, mbpsErr := NormalizeHysteriaMbps(*mbps)
	if mbpsErr == nil && parsedMbps > 0 {
		if rawText != "" {
			if _, err := NormalizeHysteriaRate(rawText, implicit); err != nil {
				warnings = append(warnings, preserveLegacyHysteriaValue(node, direction, direction, rawText)...)
			}
		}
		*text = ""
		return warnings
	}

	if mbpsErr != nil {
		shadowedMbps := *mbps
		*mbps = 0
		if rawText != "" {
			warnings = append(warnings, applyLegacyHysteriaText(node, direction, rawText, text, mbps, implicit, warnImplicit)...)
			return append(warnings, preserveLegacyHysteriaValue(node, direction, direction+"_mbps", shadowedMbps)...)
		}
		warnings = append(warnings, preserveLegacyHysteriaValue(node, direction, direction+"_mbps", shadowedMbps)...)
	}
	if rawText == "" {
		*text = ""
		return warnings
	}
	return append(warnings, applyLegacyHysteriaText(node, direction, rawText, text, mbps, implicit, warnImplicit)...)
}

func applyLegacyHysteriaText(
	node *domain.NodeIR,
	direction string,
	raw string,
	text *string,
	mbps *int,
	implicit HysteriaImplicitUnit,
	warnImplicit bool,
) []domain.Warning {
	rate, err := NormalizeHysteriaRate(raw, implicit)
	if err != nil {
		*text = ""
		*mbps = 0
		return preserveLegacyHysteriaValue(node, direction, direction, raw)
	}
	*text, *mbps = rate.Text, rate.Mbps
	if !warnImplicit || !isBareHysteriaRate(raw) {
		return nil
	}
	return []domain.Warning{{
		Code:    "parse_implicit_bandwidth_unit",
		Message: "bare Hysteria bandwidth assumed to be Mbps",
		Node:    node.Name,
		Field:   "hysteria." + direction,
		Source:  node.SourceFormat,
	}}
}

func preserveLegacyHysteriaValue(node *domain.NodeIR, direction, sourceField string, value any) []domain.Warning {
	key := "json-nodes.hysteria." + direction
	if node.Raw == nil {
		node.Raw = map[string]json.RawMessage{}
	}
	serialized, err := json.Marshal(value)
	if err != nil {
		serialized = []byte(strconv.Quote(fmt.Sprint(value)))
	}
	if existing, exists := node.Raw[key]; exists {
		if bytes.Equal(existing, serialized) {
			return nil
		}
		conflictBase := key + ".conflict." + sourceField
		key = conflictBase
		for index := 2; ; index++ {
			existing, exists = node.Raw[key]
			if !exists {
				break
			}
			if bytes.Equal(existing, serialized) {
				return nil
			}
			key = conflictBase + "." + strconv.Itoa(index)
		}
	}
	node.Raw[key] = serialized
	return []domain.Warning{{
		Code:    "parse_unknown_field",
		Message: "field preserved in NodeIR Raw",
		Node:    node.Name,
		Field:   key,
		Source:  node.SourceFormat,
	}}
}

func isBareHysteriaRate(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validateCanonicalHysteriaRate(direction, text string, mbps int) error {
	hasText := text != ""
	hasMbps := mbps != 0
	if hasText == hasMbps {
		return fmt.Errorf("hysteria %s rate must populate exactly one canonical field", direction)
	}
	if hasMbps {
		if _, err := NormalizeHysteriaMbps(mbps); err != nil {
			return fmt.Errorf("invalid hysteria %s Mbps rate: %w", direction, err)
		}
		return nil
	}

	rate, err := NormalizeHysteriaRate(text, HysteriaImplicitNone)
	if err != nil {
		return fmt.Errorf("invalid hysteria %s rate: %w", direction, err)
	}
	if rate.Text != text {
		return fmt.Errorf("hysteria %s text rate is not canonical", direction)
	}
	return nil
}
