package filedriver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	shadowrocketadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/shadowrocket"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/filekind"
	"github.com/kuuvahki-labs/sandrone/internal/inidoc"
)

type shadowrocketFileDriver struct{}

func (shadowrocketFileDriver) Descriptor() Descriptor {
	return Descriptor{
		Kind:             domain.FileKindShadowrocket,
		Description:      "Compile a node-free Shadowrocket INI configuration for separately managed subscriptions.",
		MediaType:        "text/plain; charset=utf-8",
		Syntax:           "ini",
		DefaultExtension: ".conf",
		SettingsPrototype: ShadowrocketFileCapabilitySettings{
			Groups: []ShadowrocketGroupSettings{}, RuleSets: []ShadowrocketRuleSetSettings{}, Rules: []string{},
		},
		SourceRules: filekind.SourceRules{
			AllowedTypes: []string{"inline", "remote"},
		},
		Defaults: map[string]any{"source": "built-in"},
		Examples: []map[string]any{{
			"name": "shadowrocket.conf", "kind": string(domain.FileKindShadowrocket),
			"config": map[string]any{
				"settings": map[string]any{
					"groups": []any{}, "rule_sets": []any{}, "rules": []any{},
				},
			},
		}},
		DefaultBase: []byte("[General]\n"),
	}
}

func (shadowrocketFileDriver) ValidateSettings(raw json.RawMessage) error {
	_, err := decodeShadowrocketFileSettings(raw)
	return err
}

func (shadowrocketFileDriver) Compile(_ context.Context, in CompileInput) ([]byte, error) {
	if len(bytes.TrimSpace(in.RenderedNodes)) > 0 {
		return nil, domain.NewError(domain.CodeInvalidArgument, `file kind "shadowrocket" does not accept rendered nodes`)
	}
	settings, err := decodeShadowrocketFileSettings(in.Settings)
	if err != nil {
		return nil, err
	}
	base, err := inidoc.Parse(in.Base)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "shadowrocket" base: parse INI`, err)
	}
	groupLines, err := compileShadowrocketGroups(settings.Groups)
	if err != nil {
		return nil, err
	}
	ruleLines, err := compileShadowrocketRules(settings.RuleSets, settings.Rules, groupLines)
	if err != nil {
		return nil, err
	}
	base.ReplaceSection("Proxy", nil)
	base.ReplaceSection("Proxy Group", groupLines)
	base.ReplaceSection("Rule", ruleLines)
	return base.Bytes(), nil
}

func compileShadowrocketGroups(groups []ShadowrocketGroupSettings) ([]string, error) {
	groupNames := make(map[string]bool, len(groups))
	for _, group := range groups {
		groupNames[group.Name] = true
	}
	validMembers := make(map[string]bool, len(groupNames))
	for name := range groupNames {
		validMembers[name] = true
	}

	lines := make([]string, 0, len(groups))
	for index, group := range groups {
		values := []string{group.Type}
		if group.Proxies != nil {
			members := []string{}
			seen := map[string]bool{}
			for _, member := range *group.Proxies {
				if !shadowrocketadapter.IsBuiltinGroupPolicy(member) && !validMembers[member] {
					return nil, shadowrocketSettingsError(fmt.Errorf("config.settings.groups[%d].proxies references unknown policy %q", index, member))
				}
				if !seen[member] {
					members = append(members, member)
					seen[member] = true
				}
			}
			if len(members) == 0 {
				return nil, shadowrocketSettingsError(fmt.Errorf("config.settings.groups[%d].proxies must resolve to at least one policy", index))
			}
			values = append(values, members...)
		} else {
			values = append(values, "policy-regex-filter="+strings.TrimSpace(*group.PolicyRegexFilter))
		}
		values = appendShadowrocketGroupOptions(values, group)
		lines = append(lines, group.Name+" = "+strings.Join(values, ","))
	}
	return lines, nil
}

func appendShadowrocketGroupOptions(values []string, group ShadowrocketGroupSettings) []string {
	if group.Interval != nil {
		values = append(values, "interval="+strconv.Itoa(*group.Interval))
	}
	if group.Timeout != nil {
		values = append(values, "timeout="+strconv.Itoa(*group.Timeout))
	}
	if group.Tolerance != nil {
		values = append(values, "tolerance="+strconv.Itoa(*group.Tolerance))
	}
	if group.Hidden != nil {
		values = append(values, "hidden="+shadowrocketBool(*group.Hidden))
	}
	return values
}

func shadowrocketBool(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func compileShadowrocketRules(ruleSets []ShadowrocketRuleSetSettings, rules []string, groupLines []string) ([]string, error) {
	sets := make(map[string]ShadowrocketRuleSetSettings, len(ruleSets))
	for _, item := range ruleSets {
		sets[item.Name] = item
	}
	policies := map[string]bool{}
	for _, line := range groupLines {
		if separator := strings.Index(line, " = "); separator > 0 {
			policies[strings.TrimSpace(line[:separator])] = true
		}
	}
	out := make([]string, 0, len(rules))
	for index, rule := range rules {
		parts := splitShadowrocketRule(rule)
		kind := strings.ToUpper(parts[0])
		if (kind == "RULE-SET" || kind == "DOMAIN-SET") && len(parts) >= 2 && !validHTTPURL(parts[1]) {
			parts[1] = sets[parts[1]].URL
		}
		if target, ok := shadowrocketRuleTarget(kind, parts); ok && !shadowrocketadapter.IsBuiltinRulePolicy(target) && !policies[target] {
			return nil, shadowrocketSettingsError(fmt.Errorf("config.settings.rules[%d] references unknown policy %q", index, target))
		}
		out = append(out, strings.Join(parts, ","))
	}
	return out, nil
}

func shadowrocketRuleTarget(kind string, parts []string) (string, bool) {
	policyIndex, known := shadowrocketRulePolicyIndex(kind)
	if known && len(parts) > policyIndex {
		return parts[policyIndex], true
	}
	return "", false
}
