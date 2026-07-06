package service

import (
	"bytes"
	"context"
	"encoding/json"

	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type mihomoFileDriver struct{}

func (mihomoFileDriver) Descriptor() typedFileDescriptor {
	return typedFileDescriptor{
		Kind:             domain.FileKindMihomo,
		MediaType:        "application/yaml",
		Syntax:           "yaml",
		DefaultExtension: ".yaml",
		NodeRenderFormat: "mihomo-proxies",
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

func (mihomoFileDriver) Compile(_ context.Context, in typedFileCompileInput) ([]byte, error) {
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
	if settings.Groups == nil {
		doc["proxy-groups"] = mihomoGroups("basic", names)
	} else {
		doc["proxy-groups"] = expandConfigGroups(settings.Groups, names)
	}
	if settings.RuleSets == nil {
		doc["rule-providers"] = mihomoRuleProviders("default")
	} else {
		ruleProviders, err := mihomoRuleProvidersFromConfig(settings.RuleSets)
		if err != nil {
			return nil, err
		}
		doc["rule-providers"] = ruleProviders
	}
	if settings.Rules == nil {
		doc["rules"] = mihomoRules()
	} else {
		rules := make([]any, len(settings.Rules))
		for i := range settings.Rules {
			rules[i] = settings.Rules[i]
		}
		doc["rules"] = rules
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "mihomo": encode config`, err)
	}
	return bytes.TrimRight(out, "\n"), nil
}
