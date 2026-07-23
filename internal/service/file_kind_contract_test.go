package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceRequiresCanonicalFileKind(t *testing.T) {
	for _, kind := range []domain.FileKind{"", "Static", " static ", "unknown"} {
		t.Run(string(kind), func(t *testing.T) {
			spec := domain.FileSpec{
				Name: "bad.txt", Kind: kind,
				Source: domain.FileSource{Type: "inline", Content: "body"},
			}
			_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})
			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
			require.ErrorContains(t, err, "file kind")
		})
	}
}

func TestServiceStaticFileRejectsConfig(t *testing.T) {
	spec := domain.FileSpec{
		Name: "bad.txt", Kind: domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "body"},
		Config: &domain.FileConfig{},
	}

	_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.ErrorContains(t, err, `file kind "static" does not allow config`)
}

func TestServiceMihomoExplicitEmptySettingsStayEmpty(t *testing.T) {
	spec := domain.FileSpec{
		Name: "empty.yaml", Kind: domain.FileKindMihomo,
		Config: &domain.FileConfig{Settings: json.RawMessage(`{
			"groups":[], "rule_sets":[], "rules":[]
		}`)},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(mustYAMLToJSON(t, result.Content), &doc))
	require.Equal(t, []any{}, doc["proxy-groups"])
	require.Equal(t, map[string]any{}, doc["rule-providers"])
	require.Equal(t, []any{}, doc["rules"])
}

func TestServiceSingBoxExplicitEmptySettingsStayEmpty(t *testing.T) {
	spec := domain.FileSpec{
		Name: "empty.json", Kind: domain.FileKindSingBox,
		Config: &domain.FileConfig{Settings: json.RawMessage(`{
			"groups":[], "rule_sets":[], "rules":[]
		}`)},
	}

	result, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(result.Content, &doc))
	outbounds := doc["outbounds"].([]any)
	require.Len(t, outbounds, 2)
	require.Equal(t, "direct", outbounds[0].(map[string]any)["tag"])
	require.Equal(t, "block", outbounds[1].(map[string]any)["tag"])
	route := doc["route"].(map[string]any)
	require.Equal(t, []any{}, route["rule_set"])
	require.Equal(t, []any{}, route["rules"])
}

func TestServiceTypedSettingsAreStrictPerKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     domain.FileKind
		settings string
		path     string
	}{
		{name: "settings null", kind: domain.FileKindMihomo, settings: `null`, path: "config.settings"},
		{name: "settings array", kind: domain.FileKindMihomo, settings: `[]`, path: "config.settings"},
		{name: "unknown field", kind: domain.FileKindMihomo, settings: `{"future":true}`, path: "config.settings.future"},
		{name: "removed adaptive count", kind: domain.FileKindMihomo, settings: `{"adaptive_groups":{"minimum_node_count":2}}`, path: "config.settings.adaptive_groups.minimum_node_count"},
		{name: "mihomo object rule", kind: domain.FileKindMihomo, settings: `{"rules":[{"outbound":"direct"}]}`, path: "config.settings.rules"},
		{name: "sing-box string rule", kind: domain.FileKindSingBox, settings: `{"rules":["MATCH,direct"]}`, path: "config.settings.rules"},
		{name: "null field", kind: domain.FileKindSingBox, settings: `{"groups":null}`, path: "config.settings.groups"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := domain.FileSpec{
				Name: "strict", Kind: test.kind,
				Config: &domain.FileConfig{Settings: json.RawMessage(test.settings)},
			}
			_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})
			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
			require.ErrorContains(t, err, string(test.kind))
			require.ErrorContains(t, err, test.path)
		})
	}
}

func TestServicePutFileValidatesBeforeStorage(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	bad := domain.FileSpec{
		Name: "bad.yaml", Kind: domain.FileKindMihomo,
		Config: &domain.FileConfig{Settings: json.RawMessage(`{"rules":null}`)},
	}

	err := svc.PutFile(context.Background(), bad)

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	files, listErr := svc.ListFiles(context.Background())
	require.NoError(t, listErr)
	require.Empty(t, files.Items)
}

func TestServicePutFileValidatesBeforeCheckingStoreAvailability(t *testing.T) {
	err := service.New().PutFile(context.Background(), domain.FileSpec{Name: "bad.txt"})

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.ErrorContains(t, err, "file kind is required")
}

func TestServiceTypedSettingsReportsNullFieldsDeterministically(t *testing.T) {
	spec := domain.FileSpec{
		Name: "bad.yaml", Kind: domain.FileKindMihomo,
		Config: &domain.FileConfig{Settings: json.RawMessage(`{"rules":null,"groups":null}`)},
	}

	_, err := service.New().GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.ErrorContains(t, err, "config.settings.groups must not be null")
}

func mustYAMLToJSON(t *testing.T, body []byte) []byte {
	t.Helper()
	var value any
	require.NoError(t, yaml.Unmarshal(body, &value))
	out, err := json.Marshal(value)
	require.NoError(t, err)
	return out
}
