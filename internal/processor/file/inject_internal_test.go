package file

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestInjectYAMLAtPathAppend(t *testing.T) {
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("proxies:\n  - name: old\n"), &root))
	list := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
		{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "name"},
			{Kind: yaml.ScalarNode, Value: "new"},
		}},
	}}
	require.NoError(t, injectYAMLAtPath(&root, "/proxies", list, "append"))
	out, err := yaml.Marshal(&root)
	require.NoError(t, err)
	require.Contains(t, string(out), "old")
	require.Contains(t, string(out), "new")
}

func TestInjectJSONAtPathReplace(t *testing.T) {
	doc := map[string]any{"route": map[string]any{"rules": []any{map[string]any{"type": "default"}}}}
	list := []any{map[string]any{"type": "direct", "tag": "d"}}
	require.NoError(t, injectJSONAtPath(doc, "/route/rules", list, "replace"))
	rules := doc["route"].(map[string]any)["rules"].([]any)
	require.Len(t, rules, 1)
	require.Equal(t, "direct", rules[0].(map[string]any)["type"])
}

func TestLoadJSONListFromMapping(t *testing.T) {
	list, err := loadJSONList([]byte(`{"outbounds":[{"tag":"a"}]}`), "/outbounds")
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestFileFormatFallback(t *testing.T) {
	require.Equal(t, "json", fileFormat("text", "json"))
	require.Equal(t, "yaml", fileFormat("unknown", "yaml"))
	require.Equal(t, "", fileFormat("unknown", "text"))
}

func TestLoadYAMLListErrors(t *testing.T) {
	_, err := loadYAMLList([]byte("not yaml"), "/proxies")
	require.Error(t, err)

	_, err = loadYAMLList([]byte(""), "/proxies")
	require.Error(t, err)

	_, err = loadYAMLList([]byte("name: not-a-list\n"), "/proxies")
	require.Error(t, err)
}

func TestLoadJSONListErrors(t *testing.T) {
	_, err := loadJSONList([]byte("{"), "/outbounds")
	require.Error(t, err)

	_, err = loadJSONList([]byte(`{"other":[]}`), "/outbounds")
	require.Error(t, err)

	_, err = loadJSONList([]byte(`{"outbounds":"x"}`), "/outbounds")
	require.Error(t, err)
}
