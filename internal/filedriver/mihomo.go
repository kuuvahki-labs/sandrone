package filedriver

import (
	"bytes"
	"context"
	"encoding/json"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/filekind"
)

type mihomoFileDriver struct{}

func (mihomoFileDriver) Descriptor() Descriptor {
	return Descriptor{
		Kind:             domain.FileKindMihomo,
		Description:      "Compile subscriptions into a complete Mihomo YAML configuration.",
		MediaType:        "application/yaml",
		Syntax:           "yaml",
		DefaultExtension: ".yaml",
		NodeRenderFormat: "mihomo-proxies",
		SettingsPrototype: MihomoFileCapabilitySettings{
			Groups: []map[string]any{}, RuleSets: []map[string]any{}, Rules: []string{},
		},
		SourceRules: filekind.SourceRules{
			AllowedTypes: []string{"inline", "remote"},
		},
		Defaults: map[string]any{"source": "built-in"},
		Examples: []map[string]any{{
			"name": "mihomo.yaml", "kind": string(domain.FileKindMihomo),
			"config": map[string]any{
				"subscriptions": []any{},
				"settings": map[string]any{
					"groups": []any{map[string]any{
						"name": "Proxy", "type": "select", "proxies": []any{"DIRECT"},
					}},
					"rule_sets": []any{map[string]any{
						"name": "private", "type": "inline", "behavior": "classical",
						"payload": []any{"DOMAIN-SUFFIX,local"},
					}},
					"rules": []any{"MATCH,DIRECT"},
				},
			},
		}},
		DefaultBase: []byte(`mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
proxies: []
proxy-groups: []
rule-providers: {}
rules: []`),
	}
}

func (mihomoFileDriver) ValidateSettings(raw json.RawMessage) error {
	_, err := decodeMihomoFileSettings(raw)
	return err
}

func (mihomoFileDriver) Compile(_ context.Context, in CompileInput) ([]byte, error) {
	settings, err := decodeMihomoFileSettings(in.Settings)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(in.Base, &doc); err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "mihomo" base: parse YAML`, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	proxies, err := renderedYAMLList(in.RenderedNodes, "proxies")
	if err != nil {
		return nil, err
	}
	names := namesFromMihomoProxies(proxies)
	doc["proxies"] = proxies
	doc["proxy-groups"] = expandConfigGroups(settings.Groups, names)
	ruleProviders, err := mihomoRuleProvidersFromConfig(settings.RuleSets)
	if err != nil {
		return nil, err
	}
	doc["rule-providers"] = ruleProviders
	rules := make([]any, len(settings.Rules))
	for i := range settings.Rules {
		rules[i] = settings.Rules[i]
	}
	doc["rules"] = rules
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "mihomo": encode config`, err)
	}
	return bytes.TrimRight(out, "\n"), nil
}
