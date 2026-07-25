package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	shadowrocketadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/shadowrocket"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/inidoc"
)

type shadowrocketFileDriver struct{}

func (shadowrocketFileDriver) Descriptor() typedFileDescriptor {
	return typedFileDescriptor{
		Kind:              domain.FileKindShadowrocket,
		Description:       "Compile subscriptions into a complete Shadowrocket INI configuration.",
		MediaType:         "text/plain; charset=utf-8",
		Syntax:            "ini",
		DefaultExtension:  ".conf",
		NodeRenderFormat:  "shadowrocket-proxies",
		SettingsPrototype: ShadowrocketFileCapabilitySettings{},
		SourceRules: FileKindSourceRules{
			AllowedTypes: []string{"inline", "remote"},
		},
		Defaults: map[string]any{"source": "built-in", "settings": map[string]any{}},
		Examples: []map[string]any{{
			"name": "shadowrocket.conf", "kind": string(domain.FileKindShadowrocket),
			"config": map[string]any{
				"subscriptions": []any{},
				"settings":      map[string]any{"groups": []any{}},
			},
		}},
		DefaultBase: []byte("[General]\n"),
	}
}

func (shadowrocketFileDriver) ValidateSettings(raw json.RawMessage) error {
	_, err := decodeShadowrocketFileSettings(raw)
	return err
}

func (shadowrocketFileDriver) Compile(_ context.Context, in typedFileCompileInput) ([]byte, error) {
	settings, err := decodeShadowrocketFileSettings(in.Settings)
	if err != nil {
		return nil, err
	}
	base, err := inidoc.Parse(in.Base)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "shadowrocket" base: parse INI`, err)
	}
	rendered, err := inidoc.Parse(in.RenderedNodes)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, `file kind "shadowrocket": parse rendered proxy section`, err)
	}
	proxyLines := rendered.SectionLines("Proxy")
	nodeNames, err := shadowrocketProxyNames(proxyLines)
	if err != nil {
		return nil, err
	}
	groupLines, err := compileShadowrocketGroups(settings.Groups, nodeNames)
	if err != nil {
		return nil, err
	}
	ruleLines, err := compileShadowrocketRules(settings.RuleSets, settings.Rules, groupLines, nodeNames)
	if err != nil {
		return nil, err
	}
	base.ReplaceSection("Proxy", proxyLines)
	base.ReplaceSection("Proxy Group", groupLines)
	base.ReplaceSection("Rule", ruleLines)
	return base.Bytes(), nil
}

func shadowrocketProxyNames(lines []string) ([]string, error) {
	names := make([]string, 0, len(lines))
	seen := map[string]bool{}
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		separator := strings.Index(line, " = ")
		if separator <= 0 {
			return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf(`file kind "shadowrocket": rendered Proxy line %d is not an assignment`, index+1))
		}
		name := strings.TrimSpace(line[:separator])
		if name == "" || seen[name] {
			return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf(`file kind "shadowrocket": rendered Proxy line %d has an invalid or duplicate name`, index+1))
		}
		if shadowrocketadapter.ConflictsWithBuiltinRulePolicy(name) {
			return nil, domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf(`file kind "shadowrocket": rendered node name on Proxy line %d conflicts with a built-in policy`, index+1))
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, nil
}

func compileShadowrocketGroups(groups []ShadowrocketGroupSettings, nodeNames []string) ([]string, error) {
	if groups == nil {
		for _, name := range nodeNames {
			if name == "Proxy" {
				return nil, domain.NewError(domain.CodeInvalidArgument, `file kind "shadowrocket": rendered node name conflicts with the default "Proxy" group`)
			}
		}
		members := append(append([]string{}, nodeNames...), "DIRECT")
		return []string{"Proxy = select," + strings.Join(members, ",")}, nil
	}
	nodeNameSet := make(map[string]bool, len(nodeNames))
	for _, name := range nodeNames {
		nodeNameSet[name] = true
	}
	groupNames := make(map[string]bool, len(groups))
	for index, group := range groups {
		if nodeNameSet[group.Name] {
			return nil, shadowrocketSettingsError(fmt.Errorf("config.settings.groups[%d].name conflicts with a rendered node", index))
		}
		groupNames[group.Name] = true
	}
	validMembers := make(map[string]bool, len(nodeNames)+len(groupNames))
	for _, name := range nodeNames {
		validMembers[name] = true
	}
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
				if member == "$nodes" {
					for _, name := range nodeNames {
						if !seen[name] {
							members = append(members, name)
							seen[name] = true
						}
					}
					continue
				}
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

func compileShadowrocketRules(ruleSets []ShadowrocketRuleSetSettings, rules []string, groupLines, nodeNames []string) ([]string, error) {
	if rules == nil {
		rules = []string{
			"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
			"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
			"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
			"GEOIP,CN,DIRECT",
			"FINAL,Proxy",
		}
	}
	sets := make(map[string]ShadowrocketRuleSetSettings, len(ruleSets))
	for _, item := range ruleSets {
		sets[item.Name] = item
	}
	policies := map[string]bool{}
	for _, name := range nodeNames {
		policies[name] = true
	}
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
