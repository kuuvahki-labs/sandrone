package filedriver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	shadowrocketadapter "github.com/kuuvahki-labs/sandrone/internal/adapter/shadowrocket"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type ShadowrocketFileSettings struct {
	AdaptiveGroups *ShadowrocketAdaptiveGroupSettings `json:"adaptive_groups,omitempty" jsonschema:"Legacy Web and HTTP compatibility metadata"`
	Groups         []ShadowrocketGroupSettings        `json:"groups,omitempty" jsonschema:"Explicit Shadowrocket proxy groups"`
	RuleSets       []ShadowrocketRuleSetSettings      `json:"rule_sets,omitempty" jsonschema:"Named remote rule-set declarations"`
	Rules          []string                           `json:"rules,omitempty" jsonschema:"Ordered Shadowrocket rules"`
}

// ShadowrocketFileCapabilitySettings is the public settings surface for capabilities.
// The real decoder remains broader for legacy HTTP and Web compatibility.
type ShadowrocketFileCapabilitySettings struct {
	Groups   []ShadowrocketGroupSettings   `json:"groups,omitempty" jsonschema:"Explicit Shadowrocket proxy groups"`
	RuleSets []ShadowrocketRuleSetSettings `json:"rule_sets,omitempty" jsonschema:"Named remote rule-set declarations"`
	Rules    []string                      `json:"rules,omitempty" jsonschema:"Ordered Shadowrocket rules"`
}

type ShadowrocketAdaptiveGroupSettings struct {
	Type    *string  `json:"type,omitempty" jsonschema:"Generated group type" enum:"select,url-test,load-balance"`
	Regions []string `json:"regions,omitempty" jsonschema:"Recognized lowercase region identifiers"`
}

type ShadowrocketGroupSettings struct {
	Name              string    `json:"name" jsonschema:"Unique assignment-safe group name"`
	Type              string    `json:"type" jsonschema:"Shadowrocket group type" enum:"select,url-test,fallback,load-balance,random"`
	Proxies           *[]string `json:"proxies,omitempty" jsonschema:"Fixed built-in policies or declared proxy groups"`
	PolicyRegexFilter *string   `json:"policy-regex-filter,omitempty" jsonschema:"Dynamic policy regular-expression filter"`
	Interval          *int      `json:"interval,omitempty" jsonschema:"Health-check interval in seconds" minimum:"1" maximum:"86400"`
	Timeout           *int      `json:"timeout,omitempty" jsonschema:"Health-check timeout in seconds" minimum:"1" maximum:"300"`
	Tolerance         *int      `json:"tolerance,omitempty" jsonschema:"Latency tolerance in milliseconds" minimum:"0" maximum:"65535"`
	Hidden            *bool     `json:"hidden,omitempty" jsonschema:"Whether the group is hidden in the client"`
}

type ShadowrocketRuleSetSettings struct {
	Name string `json:"name" jsonschema:"Unique name referenced by rules"`
	Type string `json:"type" jsonschema:"Remote rule-set type" enum:"rule-set,domain-set"`
	URL  string `json:"url" jsonschema:"Absolute HTTP or HTTPS URL"`
}

func decodeShadowrocketFileSettings(raw json.RawMessage) (ShadowrocketFileSettings, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	fields, err := strictJSONObject(raw, "config.settings")
	if err != nil {
		return ShadowrocketFileSettings{}, shadowrocketSettingsError(err)
	}
	allowed := map[string]bool{
		"adaptive_groups": true,
		"groups":          true,
		"rule_sets":       true,
		"rules":           true,
	}
	if err := rejectUnknownJSONFields(fields, allowed, "config.settings"); err != nil {
		return ShadowrocketFileSettings{}, shadowrocketSettingsError(err)
	}

	var settings ShadowrocketFileSettings
	if value, ok := fields["adaptive_groups"]; ok {
		var adaptive ShadowrocketAdaptiveGroupSettings
		if err := decodeStrictJSONObject(value, "config.settings.adaptive_groups", &adaptive,
			"type", "regions"); err != nil {
			return ShadowrocketFileSettings{}, shadowrocketSettingsError(err)
		}
		settings.AdaptiveGroups = &adaptive
	}
	if value, ok := fields["groups"]; ok {
		items, err := strictJSONArray(value, "config.settings.groups")
		if err != nil {
			return ShadowrocketFileSettings{}, shadowrocketSettingsError(err)
		}
		settings.Groups = make([]ShadowrocketGroupSettings, len(items))
		for index, item := range items {
			path := fmt.Sprintf("config.settings.groups[%d]", index)
			if err := decodeStrictJSONObject(item, path, &settings.Groups[index],
				"name", "type", "proxies", "policy-regex-filter", "interval", "timeout",
				"tolerance", "hidden"); err != nil {
				return ShadowrocketFileSettings{}, shadowrocketSettingsError(err)
			}
		}
	}
	if value, ok := fields["rule_sets"]; ok {
		items, err := strictJSONArray(value, "config.settings.rule_sets")
		if err != nil {
			return ShadowrocketFileSettings{}, shadowrocketSettingsError(err)
		}
		settings.RuleSets = make([]ShadowrocketRuleSetSettings, len(items))
		for index, item := range items {
			path := fmt.Sprintf("config.settings.rule_sets[%d]", index)
			if err := decodeStrictJSONObject(item, path, &settings.RuleSets[index], "name", "type", "url"); err != nil {
				return ShadowrocketFileSettings{}, shadowrocketSettingsError(err)
			}
		}
	}
	if value, ok := fields["rules"]; ok {
		if isJSONNull(value) {
			return ShadowrocketFileSettings{}, shadowrocketSettingsError(fmt.Errorf("config.settings.rules must not be null"))
		}
		if err := json.Unmarshal(value, &settings.Rules); err != nil {
			return ShadowrocketFileSettings{}, shadowrocketSettingsError(fmt.Errorf("config.settings.rules: expected an array of strings"))
		}
	}
	if err := validateShadowrocketSettings(settings); err != nil {
		return ShadowrocketFileSettings{}, shadowrocketSettingsError(err)
	}
	return settings, nil
}

func strictJSONObject(raw json.RawMessage, path string) (map[string]json.RawMessage, error) {
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s must not be null", path)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("%s must be an object", path)
	}
	for _, name := range sortedRawFieldNames(fields) {
		if isJSONNull(fields[name]) {
			return nil, fmt.Errorf("%s.%s must not be null", path, name)
		}
	}
	return fields, nil
}

func strictJSONArray(raw json.RawMessage, path string) ([]json.RawMessage, error) {
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s must not be null", path)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("%s must be an array", path)
	}
	for index, item := range items {
		if isJSONNull(item) {
			return nil, fmt.Errorf("%s[%d] must not be null", path, index)
		}
	}
	return items, nil
}

func decodeStrictJSONObject[T any](raw json.RawMessage, path string, out *T, allowedNames ...string) error {
	fields, err := strictJSONObject(raw, path)
	if err != nil {
		return err
	}
	allowed := make(map[string]bool, len(allowedNames))
	for _, name := range allowedNames {
		allowed[name] = true
	}
	if err := rejectUnknownJSONFields(fields, allowed, path); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		const prefix = "json: unknown field "
		if strings.HasPrefix(err.Error(), prefix) {
			name := strings.Trim(err.Error()[len(prefix):], `"`)
			return fmt.Errorf("%s.%s: unknown field", path, name)
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func rejectUnknownJSONFields(fields map[string]json.RawMessage, allowed map[string]bool, path string) error {
	for _, name := range sortedRawFieldNames(fields) {
		if !allowed[name] {
			return fmt.Errorf("%s.%s: unknown field", path, name)
		}
	}
	return nil
}

func sortedRawFieldNames(fields map[string]json.RawMessage) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func shadowrocketSettingsError(err error) error {
	return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf(`file kind "shadowrocket" %v`, err))
}

func validateShadowrocketSettings(settings ShadowrocketFileSettings) error {
	if err := validateShadowrocketAdaptiveGroups(settings.AdaptiveGroups); err != nil {
		return err
	}
	if err := validateShadowrocketGroups(settings.Groups); err != nil {
		return err
	}
	return validateShadowrocketRules(settings.RuleSets, settings.Rules)
}

func validateShadowrocketAdaptiveGroups(settings *ShadowrocketAdaptiveGroupSettings) error {
	if settings == nil {
		return nil
	}
	if settings.Type != nil {
		value := strings.TrimSpace(*settings.Type)
		if value != "select" && value != "url-test" && value != "load-balance" {
			return fmt.Errorf("config.settings.adaptive_groups.type must be select, url-test, or load-balance")
		}
	}
	allowedRegions := map[string]bool{
		"hk": true, "tw": true, "sg": true, "jp": true, "kr": true, "us": true,
		"ca": true, "uk": true, "de": true, "fr": true, "mo": true, "au": true,
		"ru": true, "th": true, "in": true, "my": true, "ph": true, "tr": true,
		"ua": true, "fi": true, "ar": true, "eg": true,
	}
	seen := map[string]bool{}
	for index, region := range settings.Regions {
		if !allowedRegions[region] {
			return fmt.Errorf("config.settings.adaptive_groups.regions[%d] is not a supported region", index)
		}
		if seen[region] {
			return fmt.Errorf("config.settings.adaptive_groups.regions[%d] is duplicated", index)
		}
		seen[region] = true
	}
	return nil
}

func validateShadowrocketGroups(groups []ShadowrocketGroupSettings) error {
	names := make(map[string]int, len(groups))
	for index := range groups {
		group := &groups[index]
		path := fmt.Sprintf("config.settings.groups[%d]", index)
		group.Name = strings.TrimSpace(group.Name)
		group.Type = strings.TrimSpace(group.Type)
		if group.Name == "" || strings.ContainsAny(group.Name, "\r\n=,") || strings.ContainsAny(group.Name[:1], "#;[") {
			return fmt.Errorf("%s.name must be a non-empty assignment-safe name", path)
		}
		if _, exists := names[group.Name]; exists {
			return fmt.Errorf("%s.name is duplicated", path)
		}
		if group.Name == "$nodes" || shadowrocketadapter.ConflictsWithBuiltinRulePolicy(group.Name) {
			return fmt.Errorf("%s.name is reserved", path)
		}
		names[group.Name] = index
		switch group.Type {
		case "select", "url-test", "fallback", "load-balance", "random":
		default:
			return fmt.Errorf("%s.type is not supported", path)
		}
		fixed := group.Proxies != nil
		filtered := group.PolicyRegexFilter != nil
		if fixed == filtered {
			return fmt.Errorf("%s must define exactly one of proxies or policy-regex-filter", path)
		}
		if fixed {
			seen := map[string]bool{}
			for memberIndex, member := range *group.Proxies {
				member = strings.TrimSpace(member)
				if member == "" {
					return fmt.Errorf("%s.proxies[%d] must not be empty", path, memberIndex)
				}
				if seen[member] {
					return fmt.Errorf("%s.proxies[%d] is duplicated", path, memberIndex)
				}
				seen[member] = true
				(*group.Proxies)[memberIndex] = member
			}
		} else {
			filter := strings.TrimSpace(*group.PolicyRegexFilter)
			if filter == "" {
				return fmt.Errorf("%s.policy-regex-filter must not be empty", path)
			}
			if strings.ContainsAny(filter, "\r\n,") {
				return fmt.Errorf("%s.policy-regex-filter must not contain a line break or comma", path)
			}
			*group.PolicyRegexFilter = filter
		}
		if group.Interval != nil && (*group.Interval < 1 || *group.Interval > 86400) {
			return fmt.Errorf("%s.interval must be between 1 and 86400", path)
		}
		if group.Timeout != nil && (*group.Timeout < 1 || *group.Timeout > 300) {
			return fmt.Errorf("%s.timeout must be between 1 and 300", path)
		}
		if group.Tolerance != nil && (*group.Tolerance < 0 || *group.Tolerance > 65535) {
			return fmt.Errorf("%s.tolerance must be between 0 and 65535", path)
		}
	}
	return validateShadowrocketGroupCycles(groups, names)
}

func validateShadowrocketGroupCycles(groups []ShadowrocketGroupSettings, names map[string]int) error {
	state := make([]uint8, len(groups))
	var visit func(int) bool
	visit = func(index int) bool {
		if state[index] == 1 {
			return true
		}
		if state[index] == 2 {
			return false
		}
		state[index] = 1
		if groups[index].Proxies != nil {
			for _, member := range *groups[index].Proxies {
				if next, ok := names[member]; ok && visit(next) {
					return true
				}
			}
		}
		state[index] = 2
		return false
	}
	for index := range groups {
		if visit(index) {
			return fmt.Errorf("config.settings.groups contains a reference cycle")
		}
	}
	return nil
}

func validateShadowrocketRules(ruleSets []ShadowrocketRuleSetSettings, rules []string) error {
	sets := make(map[string]ShadowrocketRuleSetSettings, len(ruleSets))
	for index := range ruleSets {
		item := &ruleSets[index]
		path := fmt.Sprintf("config.settings.rule_sets[%d]", index)
		item.Name = strings.TrimSpace(item.Name)
		item.Type = strings.TrimSpace(item.Type)
		item.URL = strings.TrimSpace(item.URL)
		if item.Name == "" || strings.ContainsAny(item.Name, "\r\n,") {
			return fmt.Errorf("%s.name must be a non-empty rule reference", path)
		}
		if _, exists := sets[item.Name]; exists {
			return fmt.Errorf("%s.name is duplicated", path)
		}
		if item.Type != "rule-set" && item.Type != "domain-set" {
			return fmt.Errorf("%s.type must be rule-set or domain-set", path)
		}
		if !validHTTPURL(item.URL) || strings.ContainsRune(item.URL, ',') {
			return fmt.Errorf("%s.url must be an absolute comma-free http(s) URL", path)
		}
		sets[item.Name] = *item
	}
	for index, rule := range rules {
		path := fmt.Sprintf("config.settings.rules[%d]", index)
		trimmedRule := strings.TrimSpace(rule)
		if trimmedRule == "" || strings.ContainsAny(rule, "\r\n") {
			return fmt.Errorf("%s must be a non-empty single-line rule", path)
		}
		if strings.HasPrefix(trimmedRule, "[") {
			return fmt.Errorf("%s must not start an INI section", path)
		}
		parts := splitShadowrocketRule(rule)
		kind := strings.ToUpper(parts[0])
		if shadowrocketLogicalRuleKind(kind) && !shadowrocketRuleParenthesesBalanced(rule) {
			return fmt.Errorf("%s contains unbalanced logical-rule parentheses", path)
		}
		policyIndex, known := shadowrocketRulePolicyIndex(kind)
		if known {
			if len(parts) <= policyIndex || parts[policyIndex] == "" {
				return fmt.Errorf("%s must include a non-empty policy", path)
			}
			if policyIndex == 2 && parts[1] == "" {
				return fmt.Errorf("%s must include a non-empty match value", path)
			}
		}
		if kind != "RULE-SET" && kind != "DOMAIN-SET" {
			continue
		}
		reference := strings.TrimSpace(parts[1])
		if validHTTPURL(reference) {
			continue
		}
		item, ok := sets[reference]
		if !ok {
			return fmt.Errorf("%s references unknown rule set %q", path, reference)
		}
		wantType := strings.ToLower(kind)
		if wantType != item.Type {
			return fmt.Errorf("%s references %q with type %q", path, reference, item.Type)
		}
	}
	return nil
}

func splitShadowrocketRule(rule string) []string {
	parts := make([]string, 0, 4)
	start := 0
	depth := 0
	escaped := false
	for index := 0; index < len(rule); index++ {
		if escaped {
			escaped = false
			continue
		}
		switch rule[index] {
		case '\\':
			escaped = true
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(rule[start:index]))
				start = index + 1
			}
		}
	}
	return append(parts, strings.TrimSpace(rule[start:]))
}

func shadowrocketRuleParenthesesBalanced(rule string) bool {
	depth := 0
	escaped := false
	for index := 0; index < len(rule); index++ {
		if escaped {
			escaped = false
			continue
		}
		switch rule[index] {
		case '\\':
			escaped = true
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return false
			}
			depth--
		}
	}
	return depth == 0
}

func shadowrocketLogicalRuleKind(kind string) bool {
	return kind == "AND" || kind == "NOT" || kind == "OR"
}

func shadowrocketRulePolicyIndex(kind string) (int, bool) {
	switch kind {
	case "FINAL", "MATCH":
		return 1, true
	case "DOMAIN-SUFFIX", "DOMAIN-KEYWORD", "DOMAIN-WILDCARD", "DOMAIN",
		"USER-AGENT", "URL-REGEX", "IP-CIDR", "IP-ASN", "RULE-SET", "DOMAIN-SET",
		"SCRIPT", "DST-PORT", "GEOIP", "AND", "NOT", "OR":
		return 2, true
	default:
		return 0, false
	}
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
