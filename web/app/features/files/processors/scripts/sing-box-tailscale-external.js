/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  const document = api.json.parse(input.file.content);
  if (!isObject(document)) {
    throw new Error("Sandrone sing-box Tailscale external preset requires a JSON object");
  }

  const inbounds = requiredObjectArray(
    document.inbounds,
    "Sandrone sing-box Tailscale external preset requires inbounds to be an array of objects",
  );
  const selectedIndex = selectTunIndex(inbounds);
  assertSelectedTun(selectedIndex);
  const selectedTun = inbounds[selectedIndex];
  const routeExclusions = optionalStringArray(
    selectedTun.route_exclude_address,
    "Sandrone sing-box Tailscale external preset requires TUN route_exclude_address to be an array of strings",
  );

  const dns = optionalObject(
    document.dns,
    "Sandrone sing-box Tailscale external preset requires dns to be an object",
  );
  const dnsServers = optionalObjectArray(
    dns.servers,
    "Sandrone sing-box Tailscale external preset requires dns.servers to be an array of objects",
  );
  const dnsRules = optionalObjectArray(
    dns.rules,
    "Sandrone sing-box Tailscale external preset requires dns.rules to be an array of objects",
  );
  const endpoints = optionalObjectArray(
    document.endpoints,
    "Sandrone sing-box Tailscale external preset requires endpoints to be an array of objects",
  );
  const outbounds = optionalObjectArray(
    document.outbounds,
    "Sandrone sing-box Tailscale external preset requires outbounds to be an array of objects",
  );

  for (const endpoint of endpoints) {
    if (endpoint.tag === "ts-ep") {
      throw new Error("Sandrone sing-box Tailscale external preset found incompatible endpoint tag ts-ep");
    }
  }
  for (const outbound of outbounds) {
    if (outbound.tag === "ts-ep") {
      throw new Error("Sandrone sing-box Tailscale external preset found incompatible outbound tag ts-ep");
    }
  }
  for (const server of dnsServers) {
    if (server.tag === "ts-dns" && !exactEqual(server, EXTERNAL_DNS_SERVER)) {
      throw new Error("Sandrone sing-box Tailscale external preset found incompatible DNS server tag ts-dns");
    }
  }

  const updatedInbounds = [...inbounds];
  updatedInbounds[selectedIndex] = {
    ...selectedTun,
    route_exclude_address: ensureExactStrings(routeExclusions, TAILSCALE_RANGES),
  };
  const updated = {
    ...document,
    dns: {
      ...dns,
      servers: ensureOneExactObject(dnsServers, EXTERNAL_DNS_SERVER),
      rules: ensureOneExactObject(dnsRules, EXTERNAL_DNS_RULE),
    },
    inbounds: updatedInbounds,
  };

  const content = api.json.stringify(updated);
  input.file.content = content;
  return input;
}

const TAILSCALE_RANGES = ["100.64.0.0/10", "fd7a:115c:a1e0::/48"];

const EXTERNAL_DNS_SERVER = {
  type: "udp",
  tag: "ts-dns",
  server: "100.100.100.100",
};

const EXTERNAL_DNS_RULE = {
  domain_suffix: ["ts.net"],
  action: "route",
  server: "ts-dns",
};

function assertSelectedTun(selectedIndex) {
  if (selectedIndex === -2) {
    throw new Error("Sandrone sing-box Tailscale preset found ambiguous TUN inbounds");
  }
  if (selectedIndex === -3) {
    throw new Error("Sandrone sing-box Tailscale preset tag tun-in is not a TUN inbound");
  }
  if (selectedIndex < 0) {
    throw new Error("Sandrone sing-box Tailscale preset requires a TUN inbound");
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

function ensureExactStrings(values, expectedValues) {
  const owned = new Set(expectedValues);
  const found = new Set();
  const result = [];
  for (const value of values) {
    if (!owned.has(value)) {
      result.push(value);
    } else if (!found.has(value)) {
      result.push(value);
      found.add(value);
    }
  }
  for (const value of expectedValues) {
    if (!found.has(value)) result.push(value);
  }
  return result;
}

function ensureOneExactObject(values, expected) {
  const result = [];
  let found = false;
  for (const value of values) {
    if (!exactEqual(value, expected)) {
      result.push(value);
    } else if (!found) {
      result.push(value);
      found = true;
    }
  }
  if (!found) result.push(expected);
  return result;
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

function requiredObjectArray(value, message) {
  if (!Array.isArray(value) || value.some((item) => !isObject(item))) {
    throw new Error(message);
  }
  return value;
}

function optionalObjectArray(value, message) {
  if (value === undefined) return [];
  return requiredObjectArray(value, message);
}

function optionalStringArray(value, message) {
  if (value === undefined) return [];
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string")) {
    throw new Error(message);
  }
  return value;
}

function optionalObject(value, message) {
  if (value === undefined) return {};
  if (!isObject(value)) throw new Error(message);
  return value;
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
