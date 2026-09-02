/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - preset_id: stable preset identifier; request args must not override it.
// - rules_json: JSON array of Shadowrocket rule strings inserted by this preset.
// - insert_mode: optional "top" insertion mode; request args must not override it.
function main(input, api) {
  rejectManagedRequestArgOverrides(input);
  const presetID = stringArgument(input, "preset_id");
  validateTopInsertMode(input, presetID);
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

  const anchor = firstRuleSection(ruleSections);
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

function validateTopInsertMode(input, presetID) {
  const value = input.args && input.args.insert_mode;
  if (value === "top" || value === undefined && presetID === "tailscale-native") return;
  throw new Error("Sandrone Shadowrocket ordered rule preset requires insert_mode top");
}

function safeAnchorError(presetID) {
  return new Error("Sandrone preset " + presetID + " cannot find a safe shadowrocket rule anchor");
}
