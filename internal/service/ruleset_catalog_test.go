package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestListRuleSetCatalogReturnsTargetSnapshot(t *testing.T) {
	svc := New(WithRuleSetCatalogSnapshot(testRuleSetCatalogSnapshot(t)))

	mihomo, err := svc.ListRuleSetCatalog(context.Background(), "mihomo")
	require.NoError(t, err)
	require.Equal(t, []RuleSetCatalogItem{
		{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs", RuleKind: "domain"},
		{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.mrs", RuleKind: "ip"},
	}, mihomo.Items)

	singBox, err := svc.ListRuleSetCatalog(context.Background(), "sing-box")
	require.NoError(t, err)
	require.Equal(t, []RuleSetCatalogItem{
		{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs", RuleKind: "domain"},
		{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/cn.srs", RuleKind: "ip"},
	}, singBox.Items)

	shadowrocket, err := svc.ListRuleSetCatalog(context.Background(), "shadowrocket")
	require.NoError(t, err)
	require.Equal(t, []RuleSetCatalogItem{
		{Name: "Apple/Apple_Domain", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Apple/Apple_Domain.list", RuleKind: "domain", ReferenceType: "DOMAIN-SET"},
		{Name: "ChinaIPs/ChinaIPs", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/ChinaIPs/ChinaIPs.list", RuleKind: "ip", ReferenceType: "RULE-SET"},
		{Name: "Global/Global", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Global/Global.list", RuleKind: "mixed", ReferenceType: "RULE-SET"},
	}, shadowrocket.Items)
}

func TestRuleSetCatalogReferenceTypeSerializationIsTargetSpecific(t *testing.T) {
	svc := New(WithRuleSetCatalogSnapshot(testRuleSetCatalogSnapshot(t)))
	mihomo, err := svc.ListRuleSetCatalog(context.Background(), "mihomo")
	require.NoError(t, err)
	body, err := json.Marshal(mihomo.Items)
	require.NoError(t, err)
	require.NotContains(t, string(body), "reference_type")

	shadowrocket, err := svc.ListRuleSetCatalog(context.Background(), "shadowrocket")
	require.NoError(t, err)
	body, err = json.Marshal(shadowrocket.Items)
	require.NoError(t, err)
	require.Contains(t, string(body), `"reference_type":"DOMAIN-SET"`)
	require.Contains(t, string(body), `"reference_type":"RULE-SET"`)
}

func TestListRuleSetCatalogReturnsAnIndependentSlice(t *testing.T) {
	svc := New(WithRuleSetCatalogSnapshot(testRuleSetCatalogSnapshot(t)))
	first, err := svc.ListRuleSetCatalog(context.Background(), "mihomo")
	require.NoError(t, err)
	first.Items[0].Name = "changed"

	second, err := svc.ListRuleSetCatalog(context.Background(), "mihomo")
	require.NoError(t, err)
	require.Equal(t, "geosite-cn", second.Items[0].Name)
}

func TestListRuleSetCatalogRequiresSupportedTarget(t *testing.T) {
	svc := New(WithRuleSetCatalogSnapshot(testRuleSetCatalogSnapshot(t)))
	for _, target := range []string{"", "clash", "Mihomo"} {
		_, err := svc.ListRuleSetCatalog(context.Background(), target)
		require.Error(t, err, target)
		require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), target)
		require.EqualError(t, err, "invalid_argument: rule-set catalog target must be mihomo, sing-box, or shadowrocket")
	}
}

func TestListRuleSetCatalogHonorsCanceledContext(t *testing.T) {
	svc := New(WithRuleSetCatalogSnapshot(testRuleSetCatalogSnapshot(t)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.ListRuleSetCatalog(ctx, "mihomo")
	require.ErrorIs(t, err, context.Canceled)
}

func TestLoadBuiltInRuleSetCatalogRejectsMissingAndDamagedSnapshots(t *testing.T) {
	tests := map[string][]byte{
		"missing gzip":  nil,
		"invalid gzip":  []byte("not gzip"),
		"invalid JSON":  gzipBytes(t, []byte("{")),
		"empty targets": gzipBytes(t, []byte(`{"mihomo":[],"sing-box":[],"shadowrocket":[]}`)),
		"invalid item": gzipBytes(t, []byte(`{
			"mihomo":[{"name":"geosite-cn","rule_kind":"domain"}],
			"sing-box":[{"name":"geosite-cn","url":"https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs","rule_kind":"domain"}],
			"shadowrocket":[{"name":"Global/Global","url":"https://example.test/Global.list","rule_kind":"mixed","reference_type":"RULE-SET"}]
		}`)),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadRuleSetCatalog(body)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrRuleSetCatalogUnavailable))
		})
	}
}

func TestLoadBuiltInRuleSetCatalogValidatesTargetsIndependently(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*ruleSetCatalogSnapshot)
	}{
		{name: "empty mihomo", mutate: func(snapshot *ruleSetCatalogSnapshot) { snapshot.Mihomo = nil }},
		{name: "empty sing-box", mutate: func(snapshot *ruleSetCatalogSnapshot) { snapshot.SingBox = nil }},
		{name: "empty shadowrocket", mutate: func(snapshot *ruleSetCatalogSnapshot) { snapshot.Shadowrocket = nil }},
		{name: "mihomo reference type must stay absent", mutate: func(snapshot *ruleSetCatalogSnapshot) {
			snapshot.Mihomo[0].ReferenceType = "RULE-SET"
		}},
		{name: "shadowrocket reference type is required", mutate: func(snapshot *ruleSetCatalogSnapshot) {
			snapshot.Shadowrocket[0].ReferenceType = ""
		}},
		{name: "shadowrocket reference type is closed", mutate: func(snapshot *ruleSetCatalogSnapshot) {
			snapshot.Shadowrocket[0].ReferenceType = "URL-SET"
		}},
		{name: "domain set must carry domain rules", mutate: func(snapshot *ruleSetCatalogSnapshot) {
			snapshot.Shadowrocket[0].RuleKind = "mixed"
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			snapshot := testRuleSetCatalogValue()
			testCase.mutate(&snapshot)
			body, err := json.Marshal(snapshot)
			require.NoError(t, err)
			_, err = loadRuleSetCatalog(gzipBytes(t, body))
			require.ErrorIs(t, err, ErrRuleSetCatalogUnavailable)
		})
	}
}

func TestListRuleSetCatalogMissingOrDamagedOverrideIsUnavailable(t *testing.T) {
	missing := New(func(s *Service) {
		s.catalog = func() (*ruleSetCatalogSnapshot, error) {
			return nil, unavailableCatalogError("snapshot is missing", nil)
		}
	})
	_, err := missing.ListRuleSetCatalog(context.Background(), "mihomo")
	require.ErrorIs(t, err, ErrRuleSetCatalogUnavailable)

	damaged := New(WithRuleSetCatalogSnapshot([]byte("invalid")))
	_, err = damaged.ListRuleSetCatalog(context.Background(), "mihomo")
	require.ErrorIs(t, err, ErrRuleSetCatalogUnavailable)
}

func TestGeneratedEmbeddedRuleSetCatalogIsUsableWhenPresent(t *testing.T) {
	snapshot, err := loadEmbeddedRuleSetCatalog()
	if errors.Is(err, ErrRuleSetCatalogUnavailable) {
		t.Skip("rule-set catalog is an optional generated build asset")
	}
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.Mihomo)
	require.NotEmpty(t, snapshot.SingBox)
	require.NotEmpty(t, snapshot.Shadowrocket)
}

func testRuleSetCatalogSnapshot(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(testRuleSetCatalogValue())
	require.NoError(t, err)
	return gzipBytes(t, body)
}

func testRuleSetCatalogValue() ruleSetCatalogSnapshot {
	return ruleSetCatalogSnapshot{
		Mihomo: []RuleSetCatalogItem{
			{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs", RuleKind: "domain"},
			{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.mrs", RuleKind: "ip"},
		},
		SingBox: []RuleSetCatalogItem{
			{Name: "geosite-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs", RuleKind: "domain"},
			{Name: "geoip-cn", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/cn.srs", RuleKind: "ip"},
		},
		Shadowrocket: []RuleSetCatalogItem{
			{Name: "Apple/Apple_Domain", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Apple/Apple_Domain.list", RuleKind: "domain", ReferenceType: "DOMAIN-SET"},
			{Name: "ChinaIPs/ChinaIPs", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/ChinaIPs/ChinaIPs.list", RuleKind: "ip", ReferenceType: "RULE-SET"},
			{Name: "Global/Global", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Global/Global.list", RuleKind: "mixed", ReferenceType: "RULE-SET"},
		},
	}
}

func gzipBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write(body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return compressed.Bytes()
}
