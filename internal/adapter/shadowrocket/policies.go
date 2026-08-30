// Package shadowrocket contains Shadowrocket policy helpers used by the typed
// configuration driver.
package shadowrocket

import "strings"

// IsBuiltinRulePolicy reports whether name is a documented, case-sensitive
// Shadowrocket rule policy.
func IsBuiltinRulePolicy(name string) bool {
	switch name {
	case "PROXY", "DIRECT", "TAILSCALE", "REJECT", "REJECT-DICT", "REJECT-ARRAY", "REJECT-200",
		"REJECT-IMG", "REJECT-TINYGIF", "REJECT-DROP", "REJECT-NO-DROP":
		return true
	default:
		return false
	}
}

// ConflictsWithBuiltinRulePolicy applies the stricter name-safety boundary used
// by the typed driver. DIRECT and REJECT are treated case-insensitively while
// PROXY and extended actions remain case-sensitive, preserving the conventional
// user-defined group "Proxy".
func ConflictsWithBuiltinRulePolicy(name string) bool {
	return IsBuiltinRulePolicy(name) || strings.EqualFold(name, "DIRECT") || strings.EqualFold(name, "REJECT")
}

// IsBuiltinGroupPolicy reports whether name is an intrinsic policy accepted by
// fixed proxy-group members.
func IsBuiltinGroupPolicy(name string) bool {
	return name == "PROXY" || name == "DIRECT" || name == "REJECT"
}
