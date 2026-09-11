export function singBoxGroupRegex(value: unknown): RegExp {
  if (typeof value !== "string") throw new TypeError("Group filter must be a string");
  const insensitive = value.startsWith("(?i)");
  const source = insensitive ? value.slice(4) : value;
  if (!source.trim()) throw new TypeError("Group filter must not be empty");
  return new RegExp(source, insensitive ? "i" : "");
}

export function validSingBoxGroupFilter(value: unknown): boolean {
  try {
    singBoxGroupRegex(value);
    return true;
  } catch {
    return false;
  }
}

export function singBoxGroupMembers(
  group: Record<string, unknown>,
  nodeNames: readonly string[],
): string[] {
  const members = Array.isArray(group.outbounds)
    ? group.outbounds.filter((member): member is string => typeof member === "string")
      .flatMap((member) => member === "$nodes" ? nodeNames : [member])
    : [];
  if (!("filter" in group) && !("exclude-filter" in group)) return members;
  const include = singBoxGroupRegex(group.filter);
  const exclude = "exclude-filter" in group ? singBoxGroupRegex(group["exclude-filter"]) : undefined;
  return [...new Set(members.filter((member) => include.test(member) && !exclude?.test(member)))];
}
