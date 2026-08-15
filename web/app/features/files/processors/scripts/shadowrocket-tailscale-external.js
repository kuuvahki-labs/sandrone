/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  const document = api.ini.parse(input.file.content);
  if (!isObject(document) || !Array.isArray(document.sections)) {
    throw new Error("Sandrone Shadowrocket Tailscale external preset requires an INI document");
  }

  const generalSections = namedSections(document.sections, "general");
  if (generalSections.length !== 1) {
    throw new Error("Sandrone Shadowrocket Tailscale external preset requires exactly one General section");
  }
  const ruleSections = namedSections(document.sections, "rule");
  if (ruleSections.length === 0) {
    throw new Error("Sandrone Shadowrocket Tailscale external preset requires a Rule section");
  }

  ensureGeneralRanges(generalSections[0], "skip-proxy");
  ensureGeneralRanges(generalSections[0], "tun-excluded-routes");
  ensureRules(ruleSections);

  input.file.content = api.ini.stringify(document);
  return input;
}

const TAILSCALE_RANGES = ["100.64.0.0/10", "fd7a:115c:a1e0::/48"];

const TAILSCALE_RULES = [
  "DOMAIN-SUFFIX,ts.net,DIRECT",
  "IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
  "IP-CIDR,fd7a:115c:a1e0::/48,DIRECT,no-resolve",
];

function ensureGeneralRanges(section, key) {
  const assignments = assignmentIndexes(section, key);
  if (assignments.length > 1) {
    throw new Error("Sandrone Shadowrocket Tailscale external preset found ambiguous " + key + " assignments");
  }
  if (assignments.length === 0) {
    section.lines.push(key + " = " + TAILSCALE_RANGES.join(","));
    return;
  }

  const index = assignments[0];
  const parsed = parseAssignment(section.lines[index]);
  const values = parsed.value.split(",").map((value) => value.trim()).filter(Boolean);
  section.lines[index] = key + " = " + ensureExactStrings(values, TAILSCALE_RANGES).join(",");
}

function ensureRules(ruleSections) {
  const additions = TAILSCALE_RULES.filter((candidate) => (
    !ruleSections.some((section) => section.lines.some((line) => line === candidate))
  ));
  ruleSections[0].lines.splice(0, 0, ...additions);
}

function namedSections(sections, name) {
  return sections.filter((section) => (
    isObject(section)
    && typeof section.name === "string"
    && section.name.toLowerCase() === name
    && Array.isArray(section.lines)
  ));
}

function assignmentIndexes(section, key) {
  const indexes = [];
  for (let index = 0; index < section.lines.length; index += 1) {
    const parsed = parseAssignment(section.lines[index]);
    if (parsed && parsed.key.toLowerCase() === key) indexes.push(index);
  }
  return indexes;
}

function parseAssignment(line) {
  if (typeof line !== "string") return null;
  const match = line.match(/^\s*([^#;][^=]*?)\s*=\s*(.*?)\s*$/);
  if (!match) return null;
  return { key: match[1].trim(), value: match[2].trim() };
}

function ensureExactStrings(current, expected) {
  const result = [...current];
  const normalized = new Set(current.map((value) => value.toLowerCase()));
  for (const value of expected) {
    if (!normalized.has(value.toLowerCase())) result.push(value);
  }
  return result;
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
