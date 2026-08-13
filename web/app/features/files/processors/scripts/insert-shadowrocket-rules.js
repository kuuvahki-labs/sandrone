/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  rejectManagedRequestArgOverrides(input);
  const presetID = stringArgument(input, "preset_id");
  const insertMode = optionalInsertMode(input);
  const presetRules = api.json.parse(stringArgument(input, "rules_json"));
  if (!Array.isArray(presetRules) || presetRules.some((rule) => typeof rule !== "string")) {
    throw new Error("Sandrone preset " + presetID + " requires an array of Shadowrocket rule strings");
  }

  const document = api.ini.parse(input.file.content);
  if (!isObject(document) || !Array.isArray(document.sections)) {
    throw safeAnchorError(presetID);
  }
  const ruleSections = document.sections.filter((section) => (
    isObject(section)
    && typeof section.name === "string"
    && section.name.toLowerCase() === "rule"
    && Array.isArray(section.lines)
  ));

  const additions = presetRules.filter((candidate) => (
    !ruleSections.some((section) => section.lines.some((line) => line === candidate))
  ));
  if (additions.length === 0) return input;

  const anchor = insertMode === "top"
    ? firstRuleSection(ruleSections)
    : firstPhysicalAnchor(ruleSections, [
      "IP-CIDR,10.0.0.0/8,",
      "GEOIP,CN,",
      "FINAL,",
    ]);
  if (!anchor) throw safeAnchorError(presetID);

  anchor.section.lines.splice(anchor.index, 0, ...additions);
  const content = api.ini.stringify(document);
  input.file.content = content;
  return input;
}

function rejectManagedRequestArgOverrides(input) {
  const request = isObject(input) ? input.request : null;
  const requestArgs = isObject(request) ? request.args : null;
  if (!isObject(requestArgs)) return;
  if (
    Object.prototype.hasOwnProperty.call(requestArgs, "preset_id")
    || Object.prototype.hasOwnProperty.call(requestArgs, "insert_mode")
    || Object.prototype.hasOwnProperty.call(requestArgs, "rules_json")
  ) {
    throw new Error("Sandrone preset arguments cannot be overridden by request args");
  }
}

function firstRuleSection(sections) {
  return sections.length > 0 ? { section: sections[0], index: 0 } : null;
}

function firstPhysicalAnchor(sections, prefixes) {
  for (const prefix of prefixes) {
    for (const section of sections) {
      const index = section.lines.findIndex((line) => (
        typeof line === "string" && line.startsWith(prefix)
      ));
      if (index >= 0) return { section, index };
    }
  }
  return null;
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringArgument(input, name) {
  const value = input.args && input.args[name];
  if (typeof value !== "string") {
    throw new Error("Sandrone ordered rule preset requires string arg " + name);
  }
  return value;
}

function optionalInsertMode(input) {
  const value = input.args && input.args.insert_mode;
  if (value === undefined) {
    return input.args && input.args.preset_id === "tailscale-native" ? "top" : "anchor";
  }
  if (value === "anchor") return value;
  if (value === "top") return value;
  throw new Error("Sandrone ordered rule preset requires insert_mode to be anchor or top");
}

function safeAnchorError(presetID) {
  return new Error("Sandrone preset " + presetID + " cannot find a safe shadowrocket rule anchor");
}
