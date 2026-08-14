/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  const document = api.json.parse(input.file.content);
  const authKey = optionalStringArg(input, "auth_key");
  if (!isObject(document)) {
    throw new Error("Sandrone sing-box Tailscale native preset requires a JSON object");
  }

  const inbounds = requiredObjectArray(
    document.inbounds,
    "Sandrone sing-box Tailscale native preset requires inbounds to be an array of objects",
  );
  const selectedIndex = selectTunIndex(inbounds);
  assertSelectedTun(selectedIndex);
  const selectedTun = inbounds[selectedIndex];
  const routeExclusions = optionalStringArray(
    selectedTun.route_exclude_address,
    "Sandrone sing-box Tailscale native preset requires TUN route_exclude_address to be an array of strings",
  );

  const endpoints = optionalObjectArray(
    document.endpoints,
    "Sandrone sing-box Tailscale native preset requires endpoints to be an array of objects",
  );
  const outbounds = optionalObjectArray(
    document.outbounds,
    "Sandrone sing-box Tailscale native preset requires outbounds to be an array of objects",
  );
  const dns = optionalObject(
    document.dns,
    "Sandrone sing-box Tailscale native preset requires dns to be an object",
  );
  const dnsServers = optionalObjectArray(
    dns.servers,
    "Sandrone sing-box Tailscale native preset requires dns.servers to be an array of objects",
  );
  const dnsRules = optionalObjectArray(
    dns.rules,
    "Sandrone sing-box Tailscale native preset requires dns.rules to be an array of objects",
  );
  if (!isObject(document.route)) {
    throw new Error("Sandrone sing-box Tailscale native preset requires route to be an object");
  }
  const routeRules = requiredObjectArray(
    document.route.rules,
    "Sandrone sing-box Tailscale native preset requires route.rules to be an array of objects",
  );

  for (const endpoint of endpoints) {
    if (endpoint.tag === "ts-ep" && !isCompatibleNativeEndpoint(endpoint)) {
      throw new Error("Sandrone sing-box Tailscale native preset found incompatible endpoint tag ts-ep");
    }
  }
  for (const outbound of outbounds) {
    if (outbound.tag === "ts-ep") {
      throw new Error("Sandrone sing-box Tailscale native preset found incompatible outbound tag ts-ep");
    }
  }
  for (const server of dnsServers) {
    if (server.tag === "ts-dns" && !exactEqual(server, NATIVE_DNS_SERVER)) {
      throw new Error("Sandrone sing-box Tailscale native preset found incompatible DNS server tag ts-dns");
    }
  }

  const anchorIndex = firstAnchorIndex(routeRules);
  if (anchorIndex < 0) {
    throw new Error("Sandrone preset tailscale-native cannot find a safe sing-box rule anchor");
  }

  const updatedRouteRules = routeRules.filter((rule) => !exactEqual(rule, NATIVE_ROUTE_RULE));
  const insertionIndex = routeRules
    .slice(0, anchorIndex)
    .filter((rule) => !exactEqual(rule, NATIVE_ROUTE_RULE))
    .length;
  updatedRouteRules.splice(insertionIndex, 0, NATIVE_ROUTE_RULE);

  const updatedInbounds = [...inbounds];
  updatedInbounds[selectedIndex] = {
    ...selectedTun,
    route_exclude_address: routeExclusions.filter((value) => !TAILSCALE_RANGES.includes(value)),
  };
  const updated = {
    ...document,
    endpoints: ensureOneTaggedObject(endpoints, nativeEndpoint(authKey)),
    dns: {
      ...dns,
      servers: ensureOneExactObject(dnsServers, NATIVE_DNS_SERVER),
      rules: ensureOneExactObject(dnsRules, NATIVE_DNS_RULE),
    },
    inbounds: updatedInbounds,
    route: {
      ...document.route,
      rules: updatedRouteRules,
    },
  };

  const content = api.json.stringify(updated);
  input.file.content = content;
  return input;
}

const TAILSCALE_RANGES = ["100.64.0.0/10", "fd7a:115c:a1e0::/48"];

const NATIVE_ENDPOINT = {
  type: "tailscale",
  tag: "ts-ep",
  ephemeral: false,
  accept_routes: false,
};

function nativeEndpoint(authKey) {
  return {
    ...NATIVE_ENDPOINT,
    ...(authKey ? { auth_key: authKey } : {}),
  };
}

function isCompatibleNativeEndpoint(endpoint) {
  const expectedKeys = Object.hasOwn(endpoint, "auth_key")
    ? ["accept_routes", "auth_key", "ephemeral", "tag", "type"]
    : ["accept_routes", "ephemeral", "tag", "type"];
  const keys = Object.keys(endpoint).sort();
  return keys.length === expectedKeys.length
    && keys.every((key, index) => key === expectedKeys[index])
    && endpoint.type === NATIVE_ENDPOINT.type
    && endpoint.tag === NATIVE_ENDPOINT.tag
    && endpoint.ephemeral === NATIVE_ENDPOINT.ephemeral
    && endpoint.accept_routes === NATIVE_ENDPOINT.accept_routes
    && (!Object.hasOwn(endpoint, "auth_key") || typeof endpoint.auth_key === "string");
}

const NATIVE_DNS_SERVER = {
  type: "tailscale",
  tag: "ts-dns",
  endpoint: "ts-ep",
  accept_default_resolvers: false,
};

const NATIVE_DNS_RULE = {
  ip_accept_any: true,
  server: "ts-dns",
};

const NATIVE_ROUTE_RULE = {
  preferred_by: ["ts-ep"],
  action: "route",
  outbound: "ts-ep",
};

function firstAnchorIndex(rules) {
  const privateRuleSet = rules.findIndex((rule) => containsPrivateRuleSet(rule.rule_set));
  if (privateRuleSet >= 0) return privateRuleSet;

  const privateIP = rules.findIndex((rule) => rule.ip_is_private === true);
  if (privateIP >= 0) return privateIP;

  return rules.findIndex(isMatchAllFinalRule);
}

function containsPrivateRuleSet(value) {
  if (Array.isArray(value)) return value.some((item) => item === "private");
  return value === "private";
}

function isMatchAllFinalRule(rule) {
  if (typeof rule.outbound !== "string") return false;
  return Object.keys(rule).every((key) => (
    key === "outbound" || key === "action" && rule.action === "route"
  ));
}

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

function ensureOneTaggedObject(values, expected) {
  const result = [];
  let found = false;
  for (const value of values) {
    if (value.tag !== expected.tag) {
      result.push(value);
    } else if (!found) {
      result.push(expected);
      found = true;
    }
  }
  if (!found) result.push(expected);
  return result;
}

function optionalStringArg(input, name) {
  const value = input.args && input.args[name];
  if (value === undefined || value === null || value === "") return "";
  if (typeof value !== "string") {
    throw new Error(`Sandrone sing-box Tailscale native preset requires ${name} to be a string`);
  }
  return value;
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
