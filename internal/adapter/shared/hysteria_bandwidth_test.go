package shared_test

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestNormalizeHysteriaRate(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		implicit shared.HysteriaImplicitUnit
		want     shared.HysteriaRate
	}{
		{name: "mihomo bare Mbps", raw: "55", implicit: shared.HysteriaImplicitMbps, want: shared.HysteriaRate{Mbps: 55}},
		{name: "sing-box numeric Bps", raw: "55", implicit: shared.HysteriaImplicitBps, want: shared.HysteriaRate{Text: "55 Bps"}},
		{name: "whole Mbps string", raw: " 55Mbps ", implicit: shared.HysteriaImplicitNone, want: shared.HysteriaRate{Mbps: 55}},
		{name: "non-integral Mbps", raw: "640 KBps", implicit: shared.HysteriaImplicitNone, want: shared.HysteriaRate{Text: "640 KBps"}},
		{name: "exact byte conversion", raw: "125 KBps", implicit: shared.HysteriaImplicitNone, want: shared.HysteriaRate{Mbps: 1}},
	}
	if strconv.IntSize == 32 {
		tests = append(tests, struct {
			name     string
			raw      string
			implicit shared.HysteriaImplicitUnit
			want     shared.HysteriaRate
		}{
			name: "whole Mbps exceeds int", raw: "2147483648 Mbps", implicit: shared.HysteriaImplicitNone,
			want: shared.HysteriaRate{Text: "2147483648 Mbps"},
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := shared.NormalizeHysteriaRate(test.raw, test.implicit)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
			require.True(t, got.Text == "" || got.Mbps == 0)
		})
	}
}

func TestNormalizeHysteriaRateRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		implicit shared.HysteriaImplicitUnit
	}{
		{name: "empty", raw: "", implicit: shared.HysteriaImplicitMbps},
		{name: "zero", raw: "0", implicit: shared.HysteriaImplicitMbps},
		{name: "negative", raw: "-1", implicit: shared.HysteriaImplicitMbps},
		{name: "bare number without implicit unit", raw: "10", implicit: shared.HysteriaImplicitNone},
		{name: "incorrect unit case", raw: "10 mbps", implicit: shared.HysteriaImplicitNone},
		{name: "fractional value", raw: "1.5 Mbps", implicit: shared.HysteriaImplicitNone},
		{name: "unknown unit", raw: "10 XBps", implicit: shared.HysteriaImplicitNone},
		{name: "multiplication overflow", raw: "18446744073709551615 Bps", implicit: shared.HysteriaImplicitNone},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := shared.NormalizeHysteriaRate(test.raw, test.implicit)
			require.Error(t, err)
		})
	}
}

func TestNormalizeHysteriaMbpsUsesSafeStrictBound(t *testing.T) {
	wantMax := int(^uint(0) >> 1)
	if maxForBitRate := uint64(math.MaxUint64 / 1_000_000); maxForBitRate < uint64(wantMax) {
		wantMax = int(maxForBitRate)
	}
	require.Equal(t, wantMax, shared.MaxHysteriaMbps())

	for _, value := range []any{wantMax, int64(wantMax), uint64(wantMax), float64(wantMax), json.Number(strconv.Itoa(wantMax)), strconv.Itoa(wantMax)} {
		got, err := shared.NormalizeHysteriaMbps(value)
		require.NoError(t, err)
		require.Equal(t, wantMax, got)
	}

	zero, err := shared.NormalizeHysteriaMbps(0)
	require.NoError(t, err)
	require.Zero(t, zero)
	for _, value := range []any{-1, 1.5, json.Number("1.5"), "1.5", ""} {
		_, err := shared.NormalizeHysteriaMbps(value)
		require.Error(t, err, "value %#v must not be accepted as integer Mbps", value)
	}
	if wantMax < int(^uint(0)>>1) {
		_, err := shared.NormalizeHysteriaMbps(strconv.FormatUint(uint64(wantMax)+1, 10))
		require.Error(t, err)
	}
}

func TestExactHysteriaMbps(t *testing.T) {
	for _, test := range []struct {
		name     string
		rate     string
		wantMbps int
		wantOK   bool
	}{
		{name: "exact bytes per second", rate: "125 KBps", wantMbps: 1, wantOK: true},
		{name: "inexact bytes per second", rate: "55 Bps", wantOK: false},
		{name: "malformed", rate: "fast", wantOK: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotMbps, gotOK := shared.ExactHysteriaMbps(test.rate)
			require.Equal(t, test.wantMbps, gotMbps)
			require.Equal(t, test.wantOK, gotOK)
		})
	}
}

func TestValidateCanonicalHysteriaBandwidth(t *testing.T) {
	require.NoError(t, shared.ValidateCanonicalHysteriaBandwidth(&domain.HysteriaOptions{
		UpMbps: 55, Down: "640 KBps",
	}))
	for _, options := range []*domain.HysteriaOptions{
		nil,
		{Up: "55", DownMbps: 100},
		{Up: "125 KBps", DownMbps: 100},
		{Up: "55 Mbps", UpMbps: 55, DownMbps: 100},
		{UpMbps: 55},
		{UpMbps: -1, DownMbps: 100},
	} {
		require.Error(t, shared.ValidateCanonicalHysteriaBandwidth(options))
	}
	max := shared.MaxHysteriaMbps()
	require.NoError(t, shared.ValidateCanonicalHysteriaBandwidth(&domain.HysteriaOptions{UpMbps: max, DownMbps: max}))
	if max < int(^uint(0)>>1) {
		require.Error(t, shared.ValidateCanonicalHysteriaBandwidth(&domain.HysteriaOptions{UpMbps: max + 1, DownMbps: max}))
	}
}

func TestNormalizeLegacyHysteriaBandwidthUsesSourceProvenance(t *testing.T) {
	tests := []struct {
		source string
		want   domain.HysteriaOptions
		warn   bool
	}{
		{source: "mihomo", want: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}},
		{source: "uri", want: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}},
		{source: "sing-box", want: domain.HysteriaOptions{Up: "55 Bps", Down: "100 Bps"}},
		{source: "json-nodes", want: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}, warn: true},
		{source: "", want: domain.HysteriaOptions{UpMbps: 55, DownMbps: 100}, warn: true},
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			node := domain.NodeIR{Name: "hy", Type: domain.NodeTypeHysteria, SourceFormat: test.source, Hysteria: &domain.HysteriaOptions{Up: "55", Down: "100"}}
			warnings := shared.NormalizeLegacyHysteriaBandwidth(&node)
			require.Equal(t, test.want, *node.Hysteria)
			require.Equal(t, test.warn, len(warnings) == 2)
			for _, warning := range warnings {
				require.Equal(t, "parse_implicit_bandwidth_unit", warning.Code)
				require.Equal(t, "bare Hysteria bandwidth assumed to be Mbps", warning.Message)
				require.Equal(t, "hy", warning.Node)
			}
		})
	}
}

func TestNormalizeLegacyHysteriaBandwidthUsesSourceSpecificPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   domain.HysteriaOptions
	}{
		{
			name:   "sing-box text wins",
			source: "sing-box",
			want:   domain.HysteriaOptions{Up: "640 KBps", DownMbps: 1000},
		},
		{
			name:   "mihomo Mbps wins",
			source: "mihomo",
			want:   domain.HysteriaOptions{UpMbps: 20, DownMbps: 100},
		},
		{
			name:   "unknown Mbps wins",
			source: "custom",
			want:   domain.HysteriaOptions{UpMbps: 20, DownMbps: 100},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := domain.NodeIR{
				Name: "hy", Type: domain.NodeTypeHysteria, SourceFormat: test.source,
				Hysteria: &domain.HysteriaOptions{Up: "640KBps", UpMbps: 20, Down: "1Gbps", DownMbps: 100},
			}
			warnings := shared.NormalizeLegacyHysteriaBandwidth(&node)
			require.Empty(t, warnings)
			require.Equal(t, test.want, *node.Hysteria)
		})
	}
}

func TestNormalizeLegacyHysteriaBandwidthPreservesInvalidInput(t *testing.T) {
	node := domain.NodeIR{
		Name: "hy", Type: domain.NodeTypeHysteria, SourceFormat: "json-nodes",
		Hysteria: &domain.HysteriaOptions{Up: "bad", Down: "-1"},
		Raw:      map[string]json.RawMessage{"existing": json.RawMessage(`true`)},
	}

	warnings := shared.NormalizeLegacyHysteriaBandwidth(&node)

	require.Equal(t, &domain.HysteriaOptions{}, node.Hysteria)
	require.JSONEq(t, `"bad"`, string(node.Raw["json-nodes.hysteria.up"]))
	require.JSONEq(t, `"-1"`, string(node.Raw["json-nodes.hysteria.down"]))
	require.Equal(t, []string{"json-nodes.hysteria.up", "json-nodes.hysteria.down"}, []string{warnings[0].Field, warnings[1].Field})
	for _, warning := range warnings {
		require.Equal(t, "parse_unknown_field", warning.Code)
		require.Equal(t, "field preserved in NodeIR Raw", warning.Message)
		require.Equal(t, "hy", warning.Node)
		require.Equal(t, "json-nodes", warning.Source)
	}
}

func TestNormalizeLegacyHysteriaBandwidthPreservesInvalidShadowedInput(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		options domain.HysteriaOptions
		want    domain.HysteriaOptions
		raw     string
	}{
		{
			name:    "Mihomo Mbps winner does not discard invalid text",
			source:  "mihomo",
			options: domain.HysteriaOptions{Up: "bad", UpMbps: 20},
			want:    domain.HysteriaOptions{UpMbps: 20},
			raw:     `"bad"`,
		},
		{
			name:    "sing-box text winner does not discard invalid Mbps",
			source:  "sing-box",
			options: domain.HysteriaOptions{Up: "55", UpMbps: -1},
			want:    domain.HysteriaOptions{Up: "55 Bps"},
			raw:     `-1`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := domain.NodeIR{
				Name: "hy", Type: domain.NodeTypeHysteria, SourceFormat: test.source,
				Hysteria: &test.options,
			}

			warnings := shared.NormalizeLegacyHysteriaBandwidth(&node)

			require.Equal(t, test.want, *node.Hysteria)
			require.JSONEq(t, test.raw, string(node.Raw["json-nodes.hysteria.up"]))
			require.Equal(t, []domain.Warning{{
				Code: "parse_unknown_field", Message: "field preserved in NodeIR Raw", Node: "hy",
				Field: "json-nodes.hysteria.up", Source: test.source,
			}}, warnings)
		})
	}
}

func TestNormalizeLegacyHysteriaBandwidthPreservesBothInvalidValues(t *testing.T) {
	for _, source := range []string{"sing-box", "mihomo", "custom"} {
		t.Run(source, func(t *testing.T) {
			node := domain.NodeIR{
				Name: "hy", Type: domain.NodeTypeHysteria, SourceFormat: source,
				Hysteria: &domain.HysteriaOptions{Up: "bad", UpMbps: -1},
			}

			warnings := shared.NormalizeLegacyHysteriaBandwidth(&node)

			require.Equal(t, &domain.HysteriaOptions{}, node.Hysteria)
			require.JSONEq(t, `"bad"`, string(node.Raw["json-nodes.hysteria.up"]))
			require.JSONEq(t, `-1`, string(node.Raw["json-nodes.hysteria.up.conflict.up_mbps"]))
			require.Equal(t, []domain.Warning{
				{Code: "parse_unknown_field", Message: "field preserved in NodeIR Raw", Node: "hy", Field: "json-nodes.hysteria.up", Source: source},
				{Code: "parse_unknown_field", Message: "field preserved in NodeIR Raw", Node: "hy", Field: "json-nodes.hysteria.up.conflict.up_mbps", Source: source},
			}, warnings)
		})
	}
}

func TestNormalizeLegacyHysteriaBandwidthDeduplicatesIdenticalRawValue(t *testing.T) {
	node := domain.NodeIR{
		Name: "same", Type: domain.NodeTypeHysteria, SourceFormat: "json-nodes",
		Hysteria: &domain.HysteriaOptions{Up: "bad"},
		Raw:      map[string]json.RawMessage{"json-nodes.hysteria.up": json.RawMessage(`"bad"`)},
	}

	warnings := shared.NormalizeLegacyHysteriaBandwidth(&node)

	require.Empty(t, warnings)
	require.Len(t, node.Raw, 1)
	require.JSONEq(t, `"bad"`, string(node.Raw["json-nodes.hysteria.up"]))
}

func TestNormalizeLegacyHysteriaBandwidthPreservesConflictingRawValuesDeterministically(t *testing.T) {
	node := domain.NodeIR{
		Name: "conflict", Type: domain.NodeTypeHysteria, SourceFormat: "mihomo",
		Hysteria: &domain.HysteriaOptions{Up: "bad", UpMbps: -1},
		Raw:      map[string]json.RawMessage{"json-nodes.hysteria.up": json.RawMessage(`"existing"`)},
	}

	warnings := shared.NormalizeLegacyHysteriaBandwidth(&node)

	require.Equal(t, &domain.HysteriaOptions{}, node.Hysteria)
	require.JSONEq(t, `"existing"`, string(node.Raw["json-nodes.hysteria.up"]))
	require.JSONEq(t, `"bad"`, string(node.Raw["json-nodes.hysteria.up.conflict.up"]))
	require.JSONEq(t, `-1`, string(node.Raw["json-nodes.hysteria.up.conflict.up_mbps"]))
	require.Len(t, warnings, 2)
	require.Equal(t, []string{
		"json-nodes.hysteria.up.conflict.up",
		"json-nodes.hysteria.up.conflict.up_mbps",
	}, []string{warnings[0].Field, warnings[1].Field})

	node.Hysteria.Up = "bad"
	node.Hysteria.UpMbps = -1
	require.Empty(t, shared.NormalizeLegacyHysteriaBandwidth(&node), "existing conflict values must not duplicate warnings")
	require.Len(t, node.Raw, 3)
}

func TestNormalizeLegacyHysteriaBandwidthRejectsOverBoundMbps(t *testing.T) {
	max := shared.MaxHysteriaMbps()
	valid := domain.NodeIR{
		Name: "max", Type: domain.NodeTypeHysteria, SourceFormat: "json-nodes",
		Hysteria: &domain.HysteriaOptions{UpMbps: max, DownMbps: max},
	}
	require.Empty(t, shared.NormalizeLegacyHysteriaBandwidth(&valid))
	require.Equal(t, &domain.HysteriaOptions{UpMbps: max, DownMbps: max}, valid.Hysteria)

	if max == int(^uint(0)>>1) {
		return
	}
	over := domain.NodeIR{
		Name: "over", Type: domain.NodeTypeHysteria, SourceFormat: "json-nodes",
		Hysteria: &domain.HysteriaOptions{UpMbps: max + 1, DownMbps: max},
	}
	warnings := shared.NormalizeLegacyHysteriaBandwidth(&over)
	require.Zero(t, over.Hysteria.UpMbps)
	require.Equal(t, max, over.Hysteria.DownMbps)
	require.JSONEq(t, strconv.Itoa(max+1), string(over.Raw["json-nodes.hysteria.up"]))
	require.Equal(t, []string{"json-nodes.hysteria.up"}, []string{warnings[0].Field})
}
