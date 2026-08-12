/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  const presetID = stringArgument(input, "preset_id");
  const presetRules = api.json.parse(stringArgument(input, "rules_json"));
  if (!Array.isArray(presetRules) || presetRules.some((rule) => !isObject(rule))) {
    throw new Error("Sandrone preset " + presetID + " requires an array of sing-box rule objects");
  }

  const document = api.json.parse(input.file.content);
  if (!isObject(document) || !isObject(document.route) || !Array.isArray(document.route.rules)) {
    throw safeAnchorError(presetID);
  }

  const additions = presetRules.filter((candidate) => (
    !document.route.rules.some((rule) => exactEqual(rule, candidate))
  ));
  if (additions.length === 0) return input;

  const anchorIndex = firstAnchorIndex(document.route.rules);
  if (anchorIndex < 0) throw safeAnchorError(presetID);

  document.route.rules.splice(anchorIndex, 0, ...additions);
  const content = api.json.stringify(document);
  input.file.content = content;
  return input;
}

function firstAnchorIndex(rules) {
  const privateRuleSet = rules.findIndex((rule) => (
    isObject(rule) && containsPrivateRuleSet(rule.rule_set)
  ));
  if (privateRuleSet >= 0) return privateRuleSet;

  const privateIP = rules.findIndex((rule) => (
    isObject(rule) && rule.ip_is_private === true
  ));
  if (privateIP >= 0) return privateIP;

  return rules.findIndex(isMatchAllFinalRule);
}

function containsPrivateRuleSet(value) {
  if (Array.isArray(value)) return value.some((item) => item === "private");
  return value === "private";
}

function isMatchAllFinalRule(rule) {
  if (!isObject(rule) || typeof rule.outbound !== "string") return false;
  return Object.keys(rule).every((key) => (
    key === "outbound" || key === "action" && rule.action === "route"
  ));
}

function exactEqual(left, right) {
  if (left === right) return true;
  if (Array.isArray(left) || Array.isArray(right)) {
    if (!Array.isArray(left) || !Array.isArray(right) || left.length !== right.length) return false;
    return left.every((value, index) => exactEqual(value, right[index]));
  }
  if (!isObject(left) || !isObject(right)) return false;
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);
  if (leftKeys.length !== rightKeys.length) return false;
  return leftKeys.every((key) => (
    Object.prototype.hasOwnProperty.call(right, key) && exactEqual(left[key], right[key])
  ));
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
  return new Error("Sandrone preset " + presetID + " cannot find a safe sing-box rule anchor");
}
