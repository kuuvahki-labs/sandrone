package shared_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type testStringer string

func (s testStringer) String() string {
	return "stringer:" + string(s)
}

func TestStringValue(t *testing.T) {
	require.Equal(t, "plain", shared.StringValue("plain"))
	require.Equal(t, "stringer:value", shared.StringValue(testStringer("value")))
	require.Empty(t, shared.StringValue(nil))
	require.Equal(t, "42", shared.StringValue(42))
}

func TestIntAndUint16Value(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want int
	}{
		{name: "int", in: 1, want: 1},
		{name: "int64", in: int64(2), want: 2},
		{name: "uint64", in: uint64(3), want: 3},
		{name: "float64", in: float64(4.8), want: 4},
		{name: "json number", in: json.Number("5"), want: 5},
		{name: "string", in: "6", want: 6},
		{name: "empty string", in: "", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := shared.IntValue(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}

	_, err := shared.IntValue(json.Number("bad"))
	require.Error(t, err)
	_, err = shared.IntValue(struct{}{})
	require.ErrorContains(t, err, "unsupported number type")

	port, err := shared.Uint16Value("65535")
	require.NoError(t, err)
	require.Equal(t, uint16(65535), port)

	_, err = shared.Uint16Value(-1)
	require.ErrorContains(t, err, "port out of range")
	_, err = shared.Uint16Value(65536)
	require.ErrorContains(t, err, "port out of range")
}

func TestBoolValue(t *testing.T) {
	for _, value := range []any{true, "true", "1", "yes", "y", "on", 1, float64(1)} {
		require.True(t, shared.BoolValue(value), "expected truthy for %#v", value)
	}
	for _, value := range []any{false, "false", "0", "no", 0, float64(0), nil, struct{}{}} {
		require.False(t, shared.BoolValue(value), "expected falsey for %#v", value)
	}
}

func TestStringSliceValue(t *testing.T) {
	in := []string{"a", "b"}
	got := shared.StringSliceValue(in)
	got[0] = "changed"
	require.Equal(t, []string{"a", "b"}, in)

	require.Equal(t, []string{"a", "1"}, shared.StringSliceValue([]any{"a", 1, ""}))
	require.Equal(t, []string{"one"}, shared.StringSliceValue("one"))
	require.Nil(t, shared.StringSliceValue(""))
	require.Equal(t, []string{"7"}, shared.StringSliceValue(7))
	require.Nil(t, shared.StringSliceValue(nil))
}

func TestStringMapValue(t *testing.T) {
	in := map[string]string{"a": "1"}
	got := shared.StringMapValue(in)
	got["a"] = "changed"
	require.Equal(t, "1", in["a"])

	require.Equal(t, map[string]string{"a": "1", "b": "true"}, shared.StringMapValue(map[string]any{"a": 1, "b": true}))
	require.Equal(t, map[string]string{"1": "one"}, shared.StringMapValue(map[any]any{1: "one"}))
	require.Nil(t, shared.StringMapValue(nil))
	require.Nil(t, shared.StringMapValue("bad"))
}

func TestAnyMapValue(t *testing.T) {
	in := map[string]any{"a": 1}
	got := shared.AnyMapValue(in)
	got["a"] = 2
	require.Equal(t, 1, in["a"])

	require.Equal(t, map[string]any{"a": "1"}, shared.AnyMapValue(map[string]string{"a": "1"}))
	require.Equal(t, map[string]any{"1": "one"}, shared.AnyMapValue(map[any]any{1: "one"}))
	require.Nil(t, shared.AnyMapValue(nil))
	require.Nil(t, shared.AnyMapValue("bad"))
}

func TestRawHelpers(t *testing.T) {
	require.JSONEq(t, `123`, string(shared.RawNumberOrString("123")))
	require.JSONEq(t, `"abc"`, string(shared.RawNumberOrString("abc")))

	raw := map[string]json.RawMessage{}
	shared.AddRaw(raw, "ok", map[string]any{"a": 1})
	shared.AddRaw(raw, "nan", math.NaN())
	require.JSONEq(t, `{"a":1}`, string(raw["ok"]))
	require.JSONEq(t, `"NaN"`, string(raw["nan"]))

	shared.AddRaw(nil, "ignored", "value")

	doc := map[string]any{"b": 2, "a": 1, "known": true}
	shared.AddUnknownRaw(raw, "p.", doc, map[string]bool{"known": true})
	require.JSONEq(t, `1`, string(raw["p.a"]))
	require.JSONEq(t, `2`, string(raw["p.b"]))
	require.NotContains(t, raw, "p.known")
	shared.AddUnknownRaw(nil, "p.", doc, nil)
}

func TestSplitHostPortLoose(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantHost string
		wantPort string
	}{
		{name: "ipv6 bracket", in: "[2001:db8::1]:443", wantHost: "2001:db8::1", wantPort: "443"},
		{name: "bare ipv6 host port", in: "2001:db8::1:443", wantHost: "2001:db8::1", wantPort: "443"},
		{name: "split host port", in: "example.com:443", wantHost: "example.com", wantPort: "443"},
		{name: "loose host port", in: "example.com:bad", wantHost: "example.com", wantPort: "bad"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := shared.SplitHostPortLoose(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.wantHost, host)
			require.Equal(t, tc.wantPort, port)
		})
	}

	_, _, err := shared.SplitHostPortLoose("[2001:db8::1")
	require.ErrorContains(t, err, "invalid ipv6 host")
	_, _, err = shared.SplitHostPortLoose("[2001:db8::1]")
	require.ErrorContains(t, err, "missing port")
	_, _, err = shared.SplitHostPortLoose("example.com")
	require.ErrorContains(t, err, "missing port")
}

func TestParseURLHostPort(t *testing.T) {
	u, err := url.Parse("https://example.com/path")
	require.NoError(t, err)
	host, port, err := shared.ParseURLHostPort(u, "443")
	require.NoError(t, err)
	require.Equal(t, "example.com", host)
	require.Equal(t, uint16(443), port)

	u, err = url.Parse("https://example.com:8443/path")
	require.NoError(t, err)
	host, port, err = shared.ParseURLHostPort(u, "443")
	require.NoError(t, err)
	require.Equal(t, "example.com", host)
	require.Equal(t, uint16(8443), port)

	u = &url.URL{Host: "example.com:70000"}
	_, _, err = shared.ParseURLHostPort(u, "443")
	require.ErrorContains(t, err, "invalid port")

	u = &url.URL{}
	_, _, err = shared.ParseURLHostPort(u, "443")
	require.ErrorContains(t, err, "missing host")

	u = &url.URL{Host: "example.com"}
	_, _, err = shared.ParseURLHostPort(u, "bad")
	require.ErrorContains(t, err, "invalid port")
}

func TestURLAndWarningHelpers(t *testing.T) {
	values := url.Values{}
	values.Set("second", "two")
	require.Equal(t, "two", shared.QueryFirst(values, "first", "second"))
	require.Empty(t, shared.QueryFirst(values, "missing"))

	require.Equal(t, "node name", shared.DecodeName("node+name", "fallback"))
	require.Equal(t, "%zz", shared.DecodeName("%zz", "fallback"))
	require.Equal(t, "fallback", shared.DecodeName("", "fallback"))
	require.Equal(t, "node", shared.DecodeName("", ""))

	node := domain.NodeIR{}
	shared.EnsureRaw(&node)
	require.NotNil(t, node.Raw)
	node.Raw["a"] = json.RawMessage(`1`)
	shared.EnsureRaw(&node)
	require.JSONEq(t, `1`, string(node.Raw["a"]))

	require.Equal(t, domain.Warning{
		Code:    "code",
		Message: "message",
		Node:    "node",
		Field:   "field",
		Source:  "source",
		Target:  "target",
	}, shared.Warning("code", "message", "node", "field", "source", "target"))

	require.Equal(t, "wrapped: cause", fmt.Errorf("wrapped: %w", errors.New("cause")).Error())
}
