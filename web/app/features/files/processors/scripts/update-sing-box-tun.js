/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - operation: sing-box TUN operation identifier; request args must not override it.
function main(input, api) {
  rejectManagedRequestArgOverride(input);
  const operation = stringArgument(input, "operation");
  if (!isSupportedOperation(operation)) {
    throw new Error("Sandrone sing-box structure preset has unsupported operation " + operation);
  }

  const document = api.json.parse(input.file.content);
  if (!isObject(document)) {
    throw new Error("Sandrone sing-box structure preset requires a JSON object");
  }
  if (document.inbounds !== undefined && !Array.isArray(document.inbounds)) {
    throw new Error("Sandrone sing-box structure preset requires inbounds to be an array");
  }
  const inbounds = document.inbounds === undefined ? [] : document.inbounds;
  const selectedIndex = selectTunIndex(inbounds);
  if (selectedIndex === -2) {
    throw new Error("Sandrone sing-box structure preset found ambiguous TUN inbounds");
  }
  if (selectedIndex === -3) {
    throw new Error("Sandrone sing-box structure preset tag tun-in is not a TUN inbound");
  }
  if (selectedIndex < 0) {
    throw new Error("Sandrone sing-box structure preset requires a TUN inbound");
  }

  const selectedTun = inbounds[selectedIndex];
  const updated = { ...document };
  const updatedInbounds = [...inbounds];
  const updatedTun = { ...selectedTun };
  if (operation === "udp-p2p-eim") {
    updatedTun.endpoint_independent_nat = true;
  } else if (operation === "linux-tun-acceleration") {
    updatedTun.auto_route = true;
    updatedTun.auto_redirect = true;
  } else if (operation === "mptcp-direct") {
    updatedTun.exclude_mptcp = true;
  } else if (operation === "windows-relaxed-route") {
    updatedTun.strict_route = false;
  }
  updatedInbounds[selectedIndex] = updatedTun;
  updated.inbounds = updatedInbounds;

  const content = api.json.stringify(updated);
  input.file.content = content;
  return input;
}

function rejectManagedRequestArgOverride(input) {
  const request = isObject(input) ? input.request : null;
  const requestArgs = isObject(request) ? request.args : null;
  if (
    isObject(requestArgs)
    && Object.prototype.hasOwnProperty.call(requestArgs, "operation")
  ) {
    throw new Error("Sandrone sing-box structure preset operation cannot be overridden by request args");
  }
}

function selectTunIndex(inbounds) {
  const tagged = [];
  const tun = [];
  for (let index = 0; index < inbounds.length; index += 1) {
    const inbound = inbounds[index];
    if (!isObject(inbound)) continue;
    if (inbound.tag === "tun-in") tagged.push(index);
    if (inbound.type === "tun") tun.push(index);
  }
  if (tagged.length > 1) return -2;
  if (tagged.length === 1) {
    return inbounds[tagged[0]].type === "tun" ? tagged[0] : -3;
  }
  if (tun.length > 1) return -2;
  return tun.length === 1 ? tun[0] : -1;
}

function isSupportedOperation(value) {
  return value === "udp-p2p-eim"
    || value === "linux-tun-acceleration"
    || value === "mptcp-direct"
    || value === "windows-relaxed-route";
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringArgument(input, name) {
  const value = input.args && input.args[name];
  if (typeof value !== "string") {
    throw new Error("Sandrone sing-box structure preset requires string arg " + name);
  }
  return value;
}
