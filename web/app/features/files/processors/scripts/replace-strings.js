/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - preset_id: stable preset identifier; request args must not override it.
// - replacements: ordered array of [source, destination] string pairs.
function main(input) {
  rejectManagedRequestArgOverrides(input);
  const presetID = stringArgument(input, "preset_id");
  const replacements = input.args && input.args.replacements;
  if (!Array.isArray(replacements) || replacements.some((pair) => (
    !Array.isArray(pair)
    || pair.length !== 2
    || typeof pair[0] !== "string"
    || typeof pair[1] !== "string"
  ))) {
    throw new Error("Sandrone preset " + presetID + " replacements requires ordered [source, destination] string pairs");
  }
  if (!input.file || typeof input.file.content !== "string") return input;
  let content = input.file.content;
  for (const pair of replacements) {
    content = content.split(pair[0]).join(pair[1]);
  }
  input.file.content = content;
  return input;
}

function rejectManagedRequestArgOverrides(input) {
  const request = isObject(input) ? input.request : null;
  const requestArgs = isObject(request) ? request.args : null;
  if (!isObject(requestArgs)) return;
  if (
    Object.prototype.hasOwnProperty.call(requestArgs, "preset_id")
    || Object.prototype.hasOwnProperty.call(requestArgs, "replacements")
  ) {
    throw new Error("Sandrone preset arguments cannot be overridden by request args");
  }
}

function stringArgument(input, name) {
  const value = input.args && input.args[name];
  if (typeof value !== "string") {
    throw new Error("Sandrone string replacement preset requires string arg " + name);
  }
  return value;
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
