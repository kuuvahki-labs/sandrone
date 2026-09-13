/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - preset_id: stable preset identifier.
// - listen_addresses: one exact Tailnet IPv4 and/or IPv6 address.
// - listen_port: mixed inbound port.
// - username/password: both empty, or one complete authentication pair.
function main(input, api) {
  const managedArgs = ["preset_id", "listen_addresses", "listen_port", "username", "password"];
  assertFileContext(input, "tailnet-share", managedArgs);
  const listenAddresses = stringArrayArgument(input, "listen_addresses");
  const listenPort = portArgument(input, "listen_port");
  const username = stringArgument(input, "username");
  const password = stringArgument(input, "password");
  if ((username === "") !== (password === "")) {
    throw new Error("Sandrone sing-box tailnet-share preset requires username and password together");
  }

  const desired = [];
  let ipv4 = null;
  let ipv6 = null;
  for (const address of listenAddresses) {
    if (address.includes("/")) {
      throw invalidTailnetAddress(address);
    }
    const parsed4 = parseIPv4(address);
    if (parsed4) {
      if (!isTailnetIPv4(parsed4) || ipv4 !== null) throw invalidTailnetAddress(address);
      ipv4 = address;
      continue;
    }
    const parsed6 = parseIPv6(address);
    if (!parsed6 || !isTailnetIPv6(parsed6) || ipv6 !== null) throw invalidTailnetAddress(address);
    ipv6 = address;
  }
  if (ipv4 === null && ipv6 === null) {
    throw new Error("Sandrone sing-box tailnet-share preset requires at least one exact Tailnet IP address");
  }
  if (ipv4 !== null) desired.push(sharedInbound("tailnet-share-v4", ipv4, listenPort, username, password));
  if (ipv6 !== null) desired.push(sharedInbound("tailnet-share-v6", ipv6, listenPort, username, password));

  const document = api.json.parse(input.file.content);
  if (!isObject(document)) {
    throw new Error("Sandrone sing-box tailnet-share preset requires a JSON object");
  }
  const inbounds = optionalObjectArray(
    document.inbounds,
    "Sandrone sing-box tailnet-share preset requires inbounds to be an array of objects",
  );
  selectManagedTunIndex(inbounds);

  const ownedIndices = new Set();
  for (const tag of ["tailnet-share-v4", "tailnet-share-v6"]) {
    const matches = [];
    inbounds.forEach((inbound, index) => {
      if (inbound.tag === tag) matches.push(index);
    });
    if (matches.length > 1) {
      throw new Error("Sandrone sing-box tailnet-share preset found duplicate inbound tag " + tag);
    }
    if (matches.length === 1) {
      const inbound = inbounds[matches[0]];
      if (!isCompatibleSharedInbound(inbound, tag)) {
        throw new Error("Sandrone sing-box tailnet-share preset found incompatible inbound tag " + tag);
      }
      ownedIndices.add(matches[0]);
    }
  }

  const retained = inbounds.filter((_, index) => !ownedIndices.has(index));
  for (const candidate of desired) {
    for (const existing of retained) {
      if (listenerConflicts(candidate, existing)) {
        throw new Error("Sandrone sing-box tailnet-share preset found a conflicting listener socket");
      }
    }
  }
  const updated = { ...document, inbounds: [...retained, ...desired] };
  input.file.content = api.json.stringify(updated);
  return input;
}

function sharedInbound(tag, listen, listenPort, username, password) {
  return {
    type: "mixed",
    tag,
    listen,
    listen_port: listenPort,
    ...(username ? { users: [{ username, password }] } : {}),
  };
}

function isCompatibleSharedInbound(inbound, expectedTag) {
  const keys = Object.keys(inbound).sort();
  const expectedKeys = Object.hasOwn(inbound, "users")
    ? ["listen", "listen_port", "tag", "type", "users"]
    : ["listen", "listen_port", "tag", "type"];
  if (keys.length !== expectedKeys.length || !keys.every((key, index) => key === expectedKeys[index])) return false;
  if (inbound.type !== "mixed" || inbound.tag !== expectedTag || !isPort(inbound.listen_port)) return false;
  const address = typeof inbound.listen === "string" ? inbound.listen : "";
  if (expectedTag.endsWith("v4")) {
    const parsed = parseIPv4(address);
    if (!parsed || !isTailnetIPv4(parsed)) return false;
  } else {
    const parsed = parseIPv6(address);
    if (!parsed || !isTailnetIPv6(parsed)) return false;
  }
  if (!Object.hasOwn(inbound, "users")) return true;
  return Array.isArray(inbound.users)
    && inbound.users.length === 1
    && isObject(inbound.users[0])
    && Object.keys(inbound.users[0]).sort().join(",") === "password,username"
    && typeof inbound.users[0].username === "string"
    && inbound.users[0].username !== ""
    && typeof inbound.users[0].password === "string"
    && inbound.users[0].password !== "";
}

function listenerConflicts(candidate, existing) {
  if (!isPort(existing.listen_port) || existing.listen_port !== candidate.listen_port) return false;
  const existingListen = typeof existing.listen === "string" ? existing.listen : "";
  if (isWildcardListener(existingListen) || isWildcardListener(candidate.listen)) return true;
  const existing4 = parseIPv4(existingListen);
  const candidate4 = parseIPv4(candidate.listen);
  if (existing4 && candidate4) return equalNumberArray(existing4, candidate4);
  const existing6 = parseIPv6(existingListen);
  const candidate6 = parseIPv6(candidate.listen);
  return existing6 !== null && candidate6 !== null && equalNumberArray(existing6, candidate6);
}

function isWildcardListener(address) {
  return address === "0.0.0.0" || address === "::" || address === "[::]" || address === "";
}

function selectManagedTunIndex(inbounds) {
  const tagged = [];
  const tun = [];
  inbounds.forEach((inbound, index) => {
    if (inbound.tag === "tun-in") tagged.push(index);
    if (inbound.type === "tun") tun.push(index);
  });
  if (tagged.length > 1 || tun.length > 1) {
    throw new Error("Sandrone sing-box tailnet-share preset found ambiguous TUN inbounds");
  }
  if (tagged.length !== 1 || inbounds[tagged[0]].type !== "tun") {
    throw new Error("Sandrone sing-box tailnet-share preset requires the tun-in TUN inbound");
  }
  if (tun[0] !== tagged[0]) {
    throw new Error("Sandrone sing-box tailnet-share preset found an unmanaged TUN inbound");
  }
  return tagged[0];
}

function parseIPv4(value) {
  if (typeof value !== "string" || !/^\d+\.\d+\.\d+\.\d+$/.test(value)) return null;
  const parts = value.split(".").map(Number);
  if (parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return null;
  if (value !== parts.join(".")) return null;
  return parts;
}

function parseIPv6(value) {
  if (typeof value !== "string" || !value || value.includes("%") || value.includes("[")) return null;
  if ((value.match(/::/g) || []).length > 1) return null;
  const halves = value.split("::");
  if (halves.length > 2) return null;
  const left = parseIPv6Groups(halves[0]);
  const right = halves.length === 2 ? parseIPv6Groups(halves[1]) : [];
  if (!left || !right) return null;
  if (halves.length === 1) return left.length === 8 ? left : null;
  const missing = 8 - left.length - right.length;
  if (missing < 1) return null;
  return [...left, ...Array(missing).fill(0), ...right];
}

function parseIPv6Groups(value) {
  if (value === "") return [];
  const raw = value.split(":");
  const out = [];
  for (let index = 0; index < raw.length; index += 1) {
    const group = raw[index];
    if (group.includes(".")) {
      if (index !== raw.length - 1) return null;
      const ipv4 = parseIPv4(group);
      if (!ipv4) return null;
      out.push(ipv4[0] * 256 + ipv4[1], ipv4[2] * 256 + ipv4[3]);
    } else {
      if (!/^[0-9a-fA-F]{1,4}$/.test(group)) return null;
      out.push(parseInt(group, 16));
    }
  }
  return out;
}

function isTailnetIPv4(parts) {
  return parts[0] === 100 && parts[1] >= 64 && parts[1] <= 127;
}

function isTailnetIPv6(parts) {
  return parts[0] === 0xfd7a && parts[1] === 0x115c && parts[2] === 0xa1e0;
}

function equalNumberArray(left, right) {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function invalidTailnetAddress(address) {
  return new Error("Sandrone sing-box tailnet-share preset requires a unique exact IP in the Tailnet ranges: " + address);
}

function portArgument(input, name) {
  const value = input.args[name];
  if (!isPort(value)) throw new Error("Sandrone sing-box tailnet-share preset requires a valid listen_port");
  return value;
}

function isPort(value) {
  return Number.isInteger(value) && value >= 1 && value <= 65535;
}

function stringArgument(input, name) {
  const value = input.args[name];
  if (typeof value !== "string") throw new Error("Sandrone sing-box tailnet-share preset requires string arg " + name);
  return value;
}

function stringArrayArgument(input, name) {
  const value = input.args[name];
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string" || item === "")) {
    throw new Error("Sandrone sing-box tailnet-share preset requires string array arg " + name);
  }
  return value;
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
