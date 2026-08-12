/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  rejectManagedRequestArgOverrides(input);
  const presetID = stringArgument(input, "preset_id");
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

  const anchor = firstPhysicalAnchor(ruleSections, [
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
    || Object.prototype.hasOwnProperty.call(requestArgs, "rules_json")
  ) {
    throw new Error("Sandrone preset arguments cannot be overridden by request args");
  }
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

function safeAnchorError(presetID) {
  return new Error("Sandrone preset " + presetID + " cannot find a safe shadowrocket rule anchor");
}
