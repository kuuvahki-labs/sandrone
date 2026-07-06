package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCatalogFiltersSupportedPaths(t *testing.T) {
	catalog, err := generateCatalog(catalogInputsWithShadowrocket(t,
		[]string{
			"geo/geosite/private.mrs",
			"geo/geosite/cn.yaml",
			"geo/geosite/nested/reject.mrs",
			"geo/geoip/cn.mrs",
			"geo/geoip/classical/reject.mrs",
		},
		[]string{
			"geo/geosite/cn.srs",
			"geo/geosite/private.json",
			"geo/geosite/nested/reject.srs",
			"geo/geoip/cn.srs",
			"geo/geoip/classical/reject.srs",
		},
	))
	require.NoError(t, err)
	require.Equal(t, catalogSnapshot{
		Mihomo: []catalogItem{
			{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.mrs", RuleKind: "ip"},
			{Name: "geosite-private", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/private.mrs", RuleKind: "domain"},
		},
		SingBox: []catalogItem{
			{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/cn.srs", RuleKind: "ip"},
			{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs", RuleKind: "domain"},
		},
		Shadowrocket: []catalogItem{
			{Name: "Baseline/Baseline", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Baseline/Baseline.list", RuleKind: "mixed", ReferenceType: "RULE-SET"},
		},
	}, catalog)
}

func TestGenerateCatalogUsesOnlyTreePaths(t *testing.T) {
	catalog, err := generateCatalog(catalogInputsWithShadowrocket(t,
		[]string{"geo/geosite/not-checked-out.mrs"},
		[]string{"geo/geosite/cn.srs"},
	))
	require.NoError(t, err)
	require.Equal(t, "geosite-not-checked-out", catalog.Mihomo[0].Name)
}

func TestGenerateCatalogSortsAndDeduplicates(t *testing.T) {
	catalog, err := generateCatalog(catalogInputsWithShadowrocket(t,
		[]string{"geo/geosite/z.mrs", "geo/geosite/a.mrs", "geo/geosite/z.mrs"},
		[]string{"geo/geosite/z.srs", "geo/geosite/a.srs", "geo/geosite/z.srs"},
	))
	require.NoError(t, err)
	require.Equal(t, []string{"geosite-a", "geosite-z"}, itemNames(catalog.Mihomo))
	require.Equal(t, []string{"geosite-a", "geosite-z"}, itemNames(catalog.SingBox))
}

func TestGenerateCatalogAlwaysPrefixesTheArtifactStem(t *testing.T) {
	catalog, err := generateCatalog(catalogInputsWithShadowrocket(t,
		[]string{"geo/geosite/geosite-cn.mrs"},
		[]string{"geo/geoip/geoip-cn.srs"},
	))
	require.NoError(t, err)
	require.Equal(t, "geosite-geosite-cn", catalog.Mihomo[0].Name)
	require.Equal(t, "geoip-geoip-cn", catalog.SingBox[0].Name)
}

func TestGenerateCatalogRejectsNameCollisions(t *testing.T) {
	_, err := sortAndDeduplicate("sing-box", []catalogItem{
		{Name: "geosite-cn", URL: "https://example.test/one.srs", RuleKind: "domain"},
		{Name: "geosite-cn", URL: "https://example.test/two.srs", RuleKind: "domain"},
	})
	require.ErrorContains(t, err, `sing-box catalog name "geosite-cn" maps to multiple URLs`)
}

func TestGenerateCatalogRequiresEveryTarget(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		inputs     func(*testing.T) catalogInputs
		wantTarget string
	}{
		{
			name: "missing mihomo",
			inputs: func(t *testing.T) catalogInputs {
				return catalogInputsWithShadowrocket(t, nil, []string{"geo/geosite/cn.srs"})
			},
			wantTarget: "mihomo",
		},
		{
			name: "missing sing-box",
			inputs: func(t *testing.T) catalogInputs {
				return catalogInputsWithShadowrocket(t, []string{"geo/geosite/cn.mrs"}, nil)
			},
			wantTarget: "sing-box",
		},
		{
			name: "missing shadowrocket",
			inputs: func(*testing.T) catalogInputs {
				return catalogInputs{
					MetaCubeXMeta: []string{"geo/geosite/cn.mrs"},
					MetaCubeXSing: []string{"geo/geosite/cn.srs"},
				}
			},
			wantTarget: "shadowrocket",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := generateCatalog(testCase.inputs(t))
			require.ErrorContains(t, err, testCase.wantTarget+" catalog has no items")
		})
	}
}

func TestGenerateCatalogEscapesURLs(t *testing.T) {
	catalog, err := generateCatalog(catalogInputsWithShadowrocket(t,
		[]string{"geo/geosite/a b.mrs"},
		[]string{"geo/geosite/a b.srs"},
	))
	require.NoError(t, err)
	require.Equal(t, "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/a%20b.mrs", catalog.Mihomo[0].URL)
}

func TestShadowrocketItemsUseOnlyREADMEUsageEntries(t *testing.T) {
	root := writeShadowrocketFixture(t, map[string]string{
		"rule/Shadowrocket/A Category/README.md": `# A category

#### 使用说明
- IP Only.list，请使用RULE-SET。
- Domain Set_Domain.list，请使用DOMAIN-SET。
- IP Only.list，请使用RULE-SET。

#### 配置建议
- IgnoredAfterHeading.list，请使用RULE-SET。
`,
		"rule/Shadowrocket/A Category/IP Only.list": `# comment

IP-CIDR,192.0.2.0/24,no-resolve
IP-CIDR6,2001:db8::/32,no-resolve
`,
		"rule/Shadowrocket/A Category/Domain Set_Domain.list": `# comment

.example.com
example.net
`,
		"rule/Shadowrocket/A Category/IgnoredAfterHeading.list": "DOMAIN-SUFFIX,ignored.example\n",
		"rule/Shadowrocket/A Category/Unlisted.list":            "DOMAIN-SUFFIX,unlisted.example\n",
		"rule/Shadowrocket/Zeta/README.md": `# Zeta

#### 使用说明
- Mixed.list，请使用RULE-SET。
`,
		"rule/Shadowrocket/Zeta/Mixed.list": `DOMAIN-SUFFIX,example.org
IP-CIDR,198.51.100.0/24,no-resolve
`,
	})

	items, err := shadowrocketItems(root)
	require.NoError(t, err)
	require.Equal(t, []catalogItem{
		{
			Name:          "A Category/Domain Set_Domain",
			URL:           "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/A%20Category/Domain%20Set_Domain.list",
			RuleKind:      "domain",
			ReferenceType: "DOMAIN-SET",
		},
		{
			Name:          "A Category/IP Only",
			URL:           "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/A%20Category/IP%20Only.list",
			RuleKind:      "ip",
			ReferenceType: "RULE-SET",
		},
		{
			Name:          "Zeta/Mixed",
			URL:           "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Zeta/Mixed.list",
			RuleKind:      "mixed",
			ReferenceType: "RULE-SET",
		},
	}, items)
}

func TestShadowrocketItemsRejectClassificationMismatches(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		filename      string
		referenceType string
		content       string
		wantError     string
	}{
		{
			name:          "domain set contains typed rules",
			filename:      "Domains_Domain.list",
			referenceType: "DOMAIN-SET",
			content:       "DOMAIN-SUFFIX,example.com\n",
			wantError:     "DOMAIN-SET",
		},
		{
			name:          "rule set contains bare domains",
			filename:      "Rules.list",
			referenceType: "RULE-SET",
			content:       ".example.com\n",
			wantError:     "RULE-SET",
		},
		{
			name:          "domain filename is declared as rule set",
			filename:      "Rules_Domain.list",
			referenceType: "RULE-SET",
			content:       "DOMAIN-SUFFIX,example.com\n",
			wantError:     "Rules_Domain.list",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := writeShadowrocketFixture(t, map[string]string{
				"rule/Shadowrocket/Fixture/README.md":            "#### 使用说明\n- " + testCase.filename + "，请使用" + testCase.referenceType + "。\n",
				"rule/Shadowrocket/Fixture/" + testCase.filename: testCase.content,
			})
			_, err := shadowrocketItems(root)
			require.ErrorContains(t, err, testCase.wantError)
		})
	}
}

func TestShadowrocketItemsRejectSymlinkedArtifacts(t *testing.T) {
	root := writeShadowrocketFixture(t, map[string]string{
		"rule/Shadowrocket/Fixture/README.md": "#### 使用说明\n- Rules.list，请使用RULE-SET。\n",
	})
	outside := filepath.Join(t.TempDir(), "Rules.list")
	require.NoError(t, os.WriteFile(outside, []byte("DOMAIN-SUFFIX,example.com\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "rule/Shadowrocket/Fixture/Rules.list")))

	_, err := shadowrocketItems(root)
	require.ErrorContains(t, err, "must be a regular file")
}

func TestShadowrocketItemsRejectArtifactsReachedThroughEscapingSymlinkDirectory(t *testing.T) {
	root := writeShadowrocketFixture(t, map[string]string{
		"rule/Shadowrocket/Fixture/README.md": "#### 使用说明\n- nested/Rules.list，请使用RULE-SET。\n",
	})
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "Rules.list"), []byte("DOMAIN-SUFFIX,example.com\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "rule/Shadowrocket/Fixture/nested")))

	_, err := shadowrocketItems(root)
	require.ErrorContains(t, err, "escapes rule subtree")
}

func TestShadowrocketItemsRejectSymlinkedREADMEs(t *testing.T) {
	root := writeShadowrocketFixture(t, map[string]string{
		"rule/Shadowrocket/Fixture/Rules.list": "DOMAIN-SUFFIX,example.com\n",
	})
	outside := filepath.Join(t.TempDir(), "README.md")
	require.NoError(t, os.WriteFile(outside, []byte("#### 使用说明\n- Rules.list，请使用RULE-SET。\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "rule/Shadowrocket/Fixture/README.md")))

	_, err := shadowrocketItems(root)
	require.ErrorContains(t, err, "README.md must be a regular file")
}

func TestWriteGzipIsDeterministic(t *testing.T) {
	catalog := catalogSnapshot{Mihomo: []catalogItem{{Name: "geosite-cn", URL: "https://example.test/cn.mrs", RuleKind: "domain"}}, SingBox: []catalogItem{}}
	var first, second bytes.Buffer
	require.NoError(t, writeGzip(&first, catalog))
	require.NoError(t, writeGzip(&second, catalog))
	require.Equal(t, first.Bytes(), second.Bytes())

	reader, err := gzip.NewReader(bytes.NewReader(first.Bytes()))
	require.NoError(t, err)
	require.True(t, reader.ModTime.IsZero())
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	var decoded catalogSnapshot
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, catalog, decoded)
}

func TestRunBuildsSnapshotFromPathLists(t *testing.T) {
	directory := t.TempDir()
	meta := writePathList(t, directory, "meta.paths", "geo/geosite/cn.mrs\ngeo/geosite/cn.yaml\n")
	sing := writePathList(t, directory, "sing.paths", "geo/geosite/cn.srs\ngeo/geoip/cn.srs\n")
	shadowrocketRoot := writeMinimalShadowrocketFixture(t)
	output := filepath.Join(directory, "catalog.json.gz")
	require.NoError(t, run([]string{
		"-output", output,
		"-metacubex-meta-paths", meta,
		"-metacubex-sing-paths", sing,
		"-shadowrocket-root", shadowrocketRoot,
	}))

	body, err := os.ReadFile(output)
	require.NoError(t, err)
	reader, err := gzip.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	decodedBody, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	var catalog catalogSnapshot
	require.NoError(t, json.Unmarshal(decodedBody, &catalog))
	require.Len(t, catalog.Mihomo, 1)
	require.Len(t, catalog.SingBox, 2)
	require.Len(t, catalog.Shadowrocket, 1)
}

func TestRunRequiresEveryPathList(t *testing.T) {
	err := run([]string{"-output", filepath.Join(t.TempDir(), "catalog.json.gz")})
	require.ErrorContains(t, err, "is required")
}

func itemNames(items []catalogItem) []string {
	names := make([]string, len(items))
	for index, item := range items {
		names[index] = item.Name
	}
	return names
}

func writePathList(t *testing.T, directory, name, body string) string {
	t.Helper()
	filename := filepath.Join(directory, name)
	require.NoError(t, os.WriteFile(filename, []byte(body), 0o644))
	return filename
}

func catalogInputsWithShadowrocket(t *testing.T, meta, sing []string) catalogInputs {
	t.Helper()
	return catalogInputs{
		MetaCubeXMeta:    meta,
		MetaCubeXSing:    sing,
		ShadowrocketRoot: writeMinimalShadowrocketFixture(t),
	}
}

func writeMinimalShadowrocketFixture(t *testing.T) string {
	t.Helper()
	return writeShadowrocketFixture(t, map[string]string{
		"rule/Shadowrocket/Baseline/README.md":     "#### 使用说明\n- Baseline.list，请使用RULE-SET。\n",
		"rule/Shadowrocket/Baseline/Baseline.list": "DOMAIN-SUFFIX,example.com\n",
	})
}

func writeShadowrocketFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		filename := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
		require.NoError(t, os.WriteFile(filename, []byte(body), 0o644))
	}
	return root
}
