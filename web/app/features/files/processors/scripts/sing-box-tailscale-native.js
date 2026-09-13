/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  assertFileContext(input, "tailscale-native", ["preset_id", "auth_key"]);
  const authKey = stringArgument(input, "auth_key");
  const document = api.json.parse(input.file.content);
  if (!isObject(document)) throw new Error("Sandrone sing-box Tailscale native preset requires a JSON object");

  const inbounds = requiredObjectArray(document.inbounds, "Sandrone sing-box Tailscale native preset requires inbounds to be an array of objects");
  const tunIndex = selectManagedTunIndex(inbounds);
  const selectedTun = inbounds[tunIndex];
  const routeExclusions = optionalStringArray(selectedTun.route_exclude_address, "Sandrone sing-box Tailscale native preset requires TUN route_exclude_address to be an array of strings");
  const endpoints = optionalObjectArray(document.endpoints, "Sandrone sing-box Tailscale native preset requires endpoints to be an array of objects");
  const outbounds = optionalObjectArray(document.outbounds, "Sandrone sing-box Tailscale native preset requires outbounds to be an array of objects");
  const dns = optionalObject(document.dns, "Sandrone sing-box Tailscale native preset requires dns to be an object");
  const dnsServers = optionalObjectArray(dns.servers, "Sandrone sing-box Tailscale native preset requires dns.servers to be an array of objects");
  const dnsRules = optionalObjectArray(dns.rules, "Sandrone sing-box Tailscale native preset requires dns.rules to be an array of objects");
  const route = requiredObject(document.route, "Sandrone sing-box Tailscale native preset requires route to be an object");
  const routeRules = requiredObjectArray(route.rules, "Sandrone sing-box Tailscale native preset requires route.rules to be an array of objects");

  for (const endpoint of endpoints) {
    if (endpoint.tag === "ts-ep" && !isCompatibleNativeEndpoint(endpoint)) throw new Error("Sandrone sing-box Tailscale native preset found incompatible endpoint tag ts-ep");
  }
  for (const outbound of outbounds) {
    if (outbound.tag === "ts-ep") throw new Error("Sandrone sing-box Tailscale native preset found incompatible outbound tag ts-ep");
  }
  for (const server of dnsServers) {
    if (server.tag === "ts-dns" && !exactEqual(server, NATIVE_DNS_SERVER)) throw new Error("Sandrone sing-box Tailscale native preset found incompatible DNS server tag ts-dns");
  }

  const anchorIndex = firstAnchorIndex(routeRules);
  if (anchorIndex < 0) throw new Error("Sandrone preset tailscale-native cannot find a safe sing-box rule anchor");
  const updatedRouteRules = routeRules.filter((rule) => !exactEqual(rule, NATIVE_ROUTE_RULE));
  const routeInsertionIndex = routeRules.slice(0, anchorIndex).filter((rule) => !exactEqual(rule, NATIVE_ROUTE_RULE)).length;
  updatedRouteRules.splice(routeInsertionIndex, 0, NATIVE_ROUTE_RULE);

  const fakeTags = new Set(dnsServers.filter((server) => server.type === "fakeip" && typeof server.tag === "string").map((server) => server.tag));
  const retainedDNSRules = dnsRules.filter((rule) => !exactEqual(rule, NATIVE_DNS_RULE) && !exactEqual(rule, LEGACY_NATIVE_DNS_RULE));
  const fakeRuleIndex = retainedDNSRules.findIndex((rule) => typeof rule.server === "string" && fakeTags.has(rule.server));
  retainedDNSRules.splice(fakeRuleIndex < 0 ? retainedDNSRules.length : fakeRuleIndex, 0, NATIVE_DNS_RULE);

  const updatedInbounds = [...inbounds];
  updatedInbounds[tunIndex] = { ...selectedTun, route_exclude_address: routeExclusions.filter((value) => !TAILSCALE_RANGES.includes(value)) };
  const updated = {
    ...document,
    endpoints: ensureOneTaggedObject(endpoints, nativeEndpoint(authKey)),
    dns: { ...dns, servers: ensureOneExactObject(dnsServers, NATIVE_DNS_SERVER), rules: retainedDNSRules },
    inbounds: updatedInbounds,
    route: { ...route, rules: updatedRouteRules },
  };
  input.file.content = api.json.stringify(updated);
  return input;
}

const TAILSCALE_RANGES = ["100.64.0.0/10", "fd7a:115c:a1e0::/48"];
const NATIVE_ENDPOINT = { type: "tailscale", tag: "ts-ep", ephemeral: false, accept_routes: false };
const NATIVE_DNS_SERVER = { type: "tailscale", tag: "ts-dns", endpoint: "ts-ep", accept_default_resolvers: false };
const NATIVE_DNS_RULE = { preferred_by: "ts-dns", action: "route", server: "ts-dns" };
const LEGACY_NATIVE_DNS_RULE = { ip_accept_any: true, server: "ts-dns" };
const NATIVE_ROUTE_RULE = { preferred_by: ["ts-ep"], action: "route", outbound: "ts-ep" };

function nativeEndpoint(authKey) {
  return { ...NATIVE_ENDPOINT, ...(authKey ? { auth_key: authKey } : {}) };
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
    && endpoint.ephemeral === false
    && endpoint.accept_routes === false
    && (!Object.hasOwn(endpoint, "auth_key") || typeof endpoint.auth_key === "string");
}

function firstAnchorIndex(rules) {
  const privateRuleSet = rules.findIndex((rule) => containsPrivateRuleSet(rule.rule_set));
  if (privateRuleSet >= 0) return privateRuleSet;
  const privateIP = rules.findIndex((rule) => rule.ip_is_private === true);
  if (privateIP >= 0) return privateIP;
  return rules.findIndex(isMatchAllFinalRule);
}

function containsPrivateRuleSet(value) { return Array.isArray(value) ? value.includes("private") : value === "private"; }
function isMatchAllFinalRule(rule) { return typeof rule.outbound === "string" && Object.keys(rule).every((key) => key === "outbound" || key === "action" && rule.action === "route"); }

function selectManagedTunIndex(inbounds) {
  const tagged = [];
  const tun = [];
  inbounds.forEach((inbound, index) => {
    if (inbound.tag === "tun-in") tagged.push(index);
    if (inbound.type === "tun") tun.push(index);
  });
  if (tagged.length > 1 || tun.length > 1) throw new Error("Sandrone sing-box Tailscale preset found ambiguous TUN inbounds");
  if (tagged.length !== 1 || inbounds[tagged[0]].type !== "tun") throw new Error("Sandrone sing-box Tailscale preset requires the tun-in TUN inbound");
  if (tun[0] !== tagged[0]) throw new Error("Sandrone sing-box Tailscale preset found an unmanaged TUN inbound");
  return tagged[0];
}

function ensureOneExactObject(values, expected) {
  const result = [];
  let found = false;
  for (const value of values) {
    if (!exactEqual(value, expected)) result.push(value);
    else if (!found) { result.push(value); found = true; }
  }
  if (!found) result.push(expected);
  return result;
}

function ensureOneTaggedObject(values, expected) {
  const result = [];
  let found = false;
  for (const value of values) {
    if (value.tag !== expected.tag) result.push(value);
    else if (!found) { result.push(expected); found = true; }
  }
  if (!found) result.push(expected);
  return result;
}

function assertFileContext(input, presetID, managedArgs) {
  if (!isObject(input) || input.stage !== "file" || !isObject(input.file) || input.file.kind !== "sing-box") throw new Error("Sandrone sing-box " + presetID + " preset requires sing-box file-stage input");
  const requestArgs = isObject(input.request) && isObject(input.request.args) ? input.request.args : null;
  if (requestArgs && managedArgs.some((name) => Object.prototype.hasOwnProperty.call(requestArgs, name))) throw new Error("Sandrone preset arguments cannot be overridden by request args");
  if (!isObject(input.args) || input.args.preset_id !== presetID) throw new Error("Sandrone sing-box " + presetID + " preset requires preset_id=" + presetID);
}

function stringArgument(input, name) {
  const value = input.args[name];
  if (typeof value !== "string") throw new Error("Sandrone sing-box Tailscale native preset requires string arg " + name);
  return value;
}

function exactEqual(left, right) {
  if (left === right) return true;
  if (Array.isArray(left) || Array.isArray(right)) return Array.isArray(left) && Array.isArray(right) && left.length === right.length && left.every((value, index) => exactEqual(value, right[index]));
  if (!isObject(left) || !isObject(right)) return false;
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);
  return leftKeys.length === rightKeys.length && leftKeys.every((key) => Object.prototype.hasOwnProperty.call(right, key) && exactEqual(left[key], right[key]));
}

function requiredObject(value, message) { if (!isObject(value)) throw new Error(message); return value; }
function requiredObjectArray(value, message) { if (!Array.isArray(value) || value.some((item) => !isObject(item))) throw new Error(message); return value; }
function optionalObjectArray(value, message) { if (value === undefined) return []; return requiredObjectArray(value, message); }
function optionalStringArray(value, message) { if (value === undefined) return []; if (!Array.isArray(value) || value.some((item) => typeof item !== "string")) throw new Error(message); return value; }
function optionalObject(value, message) { if (value === undefined) return {}; return requiredObject(value, message); }
function isObject(value) { return typeof value === "object" && value !== null && !Array.isArray(value); }
