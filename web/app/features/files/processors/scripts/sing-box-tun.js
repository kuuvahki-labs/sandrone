/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - preset_id: stable preset identifier; request args must not override it.
function main(input, api) {
  assertFileContext(input, "tun", ["preset_id"]);
  const document = api.json.parse(input.file.content);
  if (!isObject(document)) {
    throw new Error("Sandrone sing-box TUN preset requires a JSON object");
  }

  const inbounds = optionalObjectArray(
    document.inbounds,
    "Sandrone sing-box TUN preset requires inbounds to be an array of objects",
  );
  const tunIndex = selectManagedTunIndex(inbounds);
  if (tunIndex < 0) {
    const updated = { ...document, inbounds: [...inbounds, CANONICAL_TUN] };
    input.file.content = api.json.stringify(updated);
  }
  return input;
}

const CANONICAL_TUN = {
  type: "tun",
  tag: "tun-in",
  address: ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
  auto_route: true,
  strict_route: true,
  route_exclude_address: [
    "10.0.0.0/8",
    "172.16.0.0/12",
    "192.168.0.0/16",
    "169.254.0.0/16",
    "fe80::/10",
    "fc00::/7",
    "224.0.0.251/32",
    "ff02::fb/128",
  ],
};

function selectManagedTunIndex(inbounds) {
  const tagged = [];
  const tun = [];
  for (let index = 0; index < inbounds.length; index += 1) {
    const inbound = inbounds[index];
    if (inbound.tag === "tun-in") tagged.push(index);
    if (inbound.type === "tun") tun.push(index);
  }
  if (tagged.length > 1 || tun.length > 1) {
    throw new Error("Sandrone sing-box TUN preset found ambiguous TUN inbounds");
  }
  if (tagged.length === 1 && inbounds[tagged[0]].type !== "tun") {
    throw new Error("Sandrone sing-box TUN preset tag tun-in is not a TUN inbound");
  }
  if (tun.length === 1 && inbounds[tun[0]].tag !== "tun-in") {
    throw new Error("Sandrone sing-box TUN preset found an unmanaged TUN inbound");
  }
  return tagged.length === 1 ? tagged[0] : -1;
}

function assertFileContext(input, presetID, managedArgs) {
  if (!isObject(input) || input.stage !== "file" || !isObject(input.file) || input.file.kind !== "sing-box") {
    throw new Error("Sandrone sing-box " + presetID + " preset requires sing-box file-stage input");
  }
  const requestArgs = isObject(input.request) && isObject(input.request.args) ? input.request.args : null;
  if (requestArgs && managedArgs.some((name) => Object.prototype.hasOwnProperty.call(requestArgs, name))) {
    throw new Error("Sandrone preset arguments cannot be overridden by request args");
  }
  if (!isObject(input.args) || input.args.preset_id !== presetID) {
    throw new Error("Sandrone sing-box " + presetID + " preset requires preset_id=" + presetID);
  }
}

function optionalObjectArray(value, message) {
  if (value === undefined) return [];
  if (!Array.isArray(value) || value.some((item) => !isObject(item))) throw new Error(message);
  return value;
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
