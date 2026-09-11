/* eslint-disable unused-imports/no-unused-vars */

// Adapt outbound configuration after the driver expands $nodes.
// Optional saved argument default_outbound explicitly selects route.final.
// Patterns use JavaScript syntax; a leading (?i) enables case-insensitive matching.
function main(input, api) {
  rejectManagedRequestArgOverrides(input);
  const defaultOutbound = defaultOutboundArgument(input);
  const document = api.json.parse(input.file.content);
  if (!document || typeof document !== "object" || Array.isArray(document)) {
    throw new Error("Sandrone sing-box outbound adaptation requires a JSON object");
  }
  if (document.outbounds !== undefined && !Array.isArray(document.outbounds)) {
    throw new Error("Sandrone sing-box outbound adaptation requires outbounds to be an array");
  }

  let changed = false;
  for (const group of document.outbounds || []) {
    if (!group || (group.type !== "selector" && group.type !== "urltest")) continue;
    if (!("filter" in group) && !("exclude-filter" in group)) continue;

    const include = compilePattern(group.filter, "filter", group.tag);
    const exclude = "exclude-filter" in group
      ? compilePattern(group["exclude-filter"], "exclude-filter", group.tag)
      : null;
    if (!Array.isArray(group.outbounds)
      || group.outbounds.some((name) => typeof name !== "string" || !name)) {
      throw groupError(group.tag, "outbounds must be an array of node names");
    }
    const members = [...new Set(group.outbounds.filter((name) => (
      include.test(name) && (!exclude || !exclude.test(name))
    )))];
    if (members.length === 0) throw groupError(group.tag, "filter matched no nodes");
    if ("default" in group && !members.includes(group.default)) {
      throw groupError(group.tag, "default is not a member after filtering");
    }
    group.outbounds = members;
    delete group.filter;
    delete group["exclude-filter"];
    changed = true;
  }

  if (defaultOutbound !== undefined) {
    if (document.endpoints !== undefined && !Array.isArray(document.endpoints)) {
      throw adaptationError("requires endpoints to be an array");
    }
    const targets = [...document.outbounds || [], ...document.endpoints || []];
    const matches = targets.filter((target) => isObject(target) && target.tag === defaultOutbound);
    if (matches.length !== 1) {
      throw adaptationError(`default_outbound [${defaultOutbound}] must match exactly one outbound or endpoint tag (found ${matches.length})`);
    }
    if (document.route !== undefined && !isObject(document.route)) {
      throw adaptationError("requires route to be an object when default_outbound is set");
    }
    if (document.route === undefined) document.route = {};
    if (document.route.final !== defaultOutbound) {
      document.route.final = defaultOutbound;
      changed = true;
    }
  }

  if (changed) input.file.content = api.json.stringify(document);
  return input;
}

function compilePattern(value, field, tag) {
  if (typeof value !== "string") throw groupError(tag, `${field} must be a non-empty string`);
  const insensitive = value.startsWith("(?i)");
  const pattern = insensitive ? value.slice(4) : value;
  if (!pattern.trim()) throw groupError(tag, `${field} must be a non-empty string`);
  try {
    return new RegExp(pattern, insensitive ? "i" : "");
  } catch (error) {
    throw groupError(tag, `invalid ${field}: ${error.message}`);
  }
}

function groupError(tag, message) {
  return new Error(`Sandrone sing-box outbound adaptation [${String(tag)}]: ${message}`);
}

function rejectManagedRequestArgOverrides(input) {
  const request = isObject(input) ? input.request : null;
  const requestArgs = isObject(request) ? request.args : null;
  if (isObject(requestArgs) && Object.prototype.hasOwnProperty.call(requestArgs, "default_outbound")) {
    throw adaptationError("default_outbound cannot be overridden by request args");
  }
}

function defaultOutboundArgument(input) {
  const value = input.args && input.args.default_outbound;
  if (value === undefined) return undefined;
  if (typeof value !== "string") throw adaptationError("default_outbound must be a string");
  return value.trim() ? value : undefined;
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function adaptationError(message) {
  return new Error(`Sandrone sing-box outbound adaptation ${message}`);
}
