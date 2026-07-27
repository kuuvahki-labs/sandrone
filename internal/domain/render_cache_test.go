package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestResourceRenderCacheTTLJSONPreservesTriState(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want *int
	}{
		{name: "inherit", body: `{}`, want: nil},
		{name: "disabled", body: `{"render_cache_ttl_seconds":0}`, want: intPointer(0)},
		{name: "custom", body: `{"render_cache_ttl_seconds":120}`, want: intPointer(120)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var subscription Subscription
			require.NoError(t, json.Unmarshal([]byte(tc.body), &subscription))
			require.Equal(t, tc.want, subscription.RenderCacheTTLSeconds)

			var file FileSpec
			require.NoError(t, json.Unmarshal([]byte(tc.body), &file))
			require.Equal(t, tc.want, file.RenderCacheTTLSeconds)
		})
	}
}

func TestResourceRenderCacheTTLYAMLPreservesTriState(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want *int
	}{
		{name: "inherit", body: `{}`, want: nil},
		{name: "disabled", body: "render_cache_ttl_seconds: 0\n", want: intPointer(0)},
		{name: "custom", body: "render_cache_ttl_seconds: 120\n", want: intPointer(120)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var subscription Subscription
			require.NoError(t, yaml.Unmarshal([]byte(tc.body), &subscription))
			require.Equal(t, tc.want, subscription.RenderCacheTTLSeconds)

			var file FileSpec
			require.NoError(t, yaml.Unmarshal([]byte(tc.body), &file))
			require.Equal(t, tc.want, file.RenderCacheTTLSeconds)
		})
	}
}

func intPointer(value int) *int {
	return &value
}
