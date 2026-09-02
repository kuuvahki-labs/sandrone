export const shadowrocketGroupBuiltinPolicies = ["PROXY", "DIRECT", "REJECT", "REJECT-DROP"] as const;

export const shadowrocketRuleBuiltinPolicies = [
  "PROXY", "DIRECT", "TAILSCALE", "REJECT", "REJECT-DICT", "REJECT-ARRAY",
  "REJECT-200", "REJECT-IMG", "REJECT-TINYGIF", "REJECT-DROP", "REJECT-NO-DROP",
] as const;

export function conflictsWithShadowrocketBuiltinRulePolicy(name: string): boolean {
  if (["DIRECT", "REJECT"].includes(name.toUpperCase())) return true;
  return (shadowrocketRuleBuiltinPolicies as readonly string[]).includes(name);
}
