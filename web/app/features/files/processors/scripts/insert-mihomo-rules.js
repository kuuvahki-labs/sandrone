/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - preset_id: stable preset identifier; request args must not override it.
// - rules_json: JSON array of Mihomo rule strings inserted by this preset.
function main(input, api) {
  rejectManagedRequestArgOverrides(input);
  const presetID = stringArgument(input, "preset_id");
  const presetRules = api.json.parse(stringArgument(input, "rules_json"));
  if (!Array.isArray(presetRules) || presetRules.some((rule) => typeof rule !== "string")) {
    throw new Error("Sandrone preset " + presetID + " requires an array of Mihomo rule strings");
  }

  const document = api.yaml.parse(input.file.content);
  if (!isObject(document) || !Array.isArray(document.rules)) {
    throw safeAnchorError(presetID);
  }

  const additions = presetRules.filter((candidate) => (
    !document.rules.some((rule) => rule === candidate)
  ));
  if (additions.length === 0) return input;

  const anchorIndex = firstAnchorIndex(document.rules, [
    "RULE-SET,private,",
    "GEOIP,CN,",
    "MATCH,",
  ]);
  if (anchorIndex < 0) throw safeAnchorError(presetID);

  document.rules.splice(anchorIndex, 0, ...additions);
  const content = api.yaml.stringify(document);
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

function firstAnchorIndex(rules, prefixes) {
  for (const prefix of prefixes) {
    const index = rules.findIndex((rule) => (
      typeof rule === "string" && rule.startsWith(prefix)
    ));
    if (index >= 0) return index;
  }
  return -1;
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
  return new Error("Sandrone preset " + presetID + " cannot find a safe mihomo rule anchor");
}
