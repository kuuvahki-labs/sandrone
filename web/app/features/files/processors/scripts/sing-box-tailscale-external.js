/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  assertFileContext(input, "tailscale-external", ["preset_id"]);
  const document = api.json.parse(input.file.content);
  if (!isObject(document)) throw new Error("Sandrone sing-box Tailscale external preset requires a JSON object");

  const inbounds = requiredObjectArray(document.inbounds, "Sandrone sing-box Tailscale external preset requires inbounds to be an array of objects");
  const tunIndex = selectManagedTunIndex(inbounds);
  const selectedTun = inbounds[tunIndex];
  const routeExclusions = optionalStringArray(selectedTun.route_exclude_address, "Sandrone sing-box Tailscale external preset requires TUN route_exclude_address to be an array of strings");
  const dns = optionalObject(document.dns, "Sandrone sing-box Tailscale external preset requires dns to be an object");
  const dnsServers = optionalObjectArray(dns.servers, "Sandrone sing-box Tailscale external preset requires dns.servers to be an array of objects");
  const dnsRules = optionalObjectArray(dns.rules, "Sandrone sing-box Tailscale external preset requires dns.rules to be an array of objects");
  const endpoints = optionalObjectArray(document.endpoints, "Sandrone sing-box Tailscale external preset requires endpoints to be an array of objects");
  const outbounds = optionalObjectArray(document.outbounds, "Sandrone sing-box Tailscale external preset requires outbounds to be an array of objects");

  for (const endpoint of endpoints) {
    if (endpoint.tag === "ts-ep") throw new Error("Sandrone sing-box Tailscale external preset found incompatible endpoint tag ts-ep");
  }
  for (const outbound of outbounds) {
    if (outbound.tag === "ts-ep") throw new Error("Sandrone sing-box Tailscale external preset found incompatible outbound tag ts-ep");
  }
  for (const server of dnsServers) {
    if (server.tag === "ts-dns" && !exactEqual(server, EXTERNAL_DNS_SERVER)) throw new Error("Sandrone sing-box Tailscale external preset found incompatible DNS server tag ts-dns");
  }

  const fakeTags = new Set(dnsServers.filter((server) => server.type === "fakeip" && typeof server.tag === "string").map((server) => server.tag));
  const ownedDNSRules = [EXTERNAL_DNS_RULE];
  if (fakeTags.size > 0) ownedDNSRules.push({ domain_suffix: ["tailscale.com"], action: "route", server: defaultRealDNSTag(dns, dnsServers) });
  const retainedRules = dnsRules.filter((rule) => !isOwnedExternalDNSRule(rule));
  const fakeRuleIndex = retainedRules.findIndex((rule) => typeof rule.server === "string" && fakeTags.has(rule.server));
  retainedRules.splice(fakeRuleIndex < 0 ? 0 : fakeRuleIndex, 0, ...ownedDNSRules);

  const updatedInbounds = [...inbounds];
  updatedInbounds[tunIndex] = { ...selectedTun, route_exclude_address: ensureExactStrings(routeExclusions, TAILSCALE_RANGES) };
  const updated = {
    ...document,
    dns: { ...dns, servers: ensureOneExactObject(dnsServers, EXTERNAL_DNS_SERVER), rules: retainedRules },
    inbounds: updatedInbounds,
  };
  input.file.content = api.json.stringify(updated);
  return input;
}

const TAILSCALE_RANGES = ["100.64.0.0/10", "fd7a:115c:a1e0::/48"];
const EXTERNAL_DNS_SERVER = { type: "udp", tag: "ts-dns", server: "100.100.100.100" };
const EXTERNAL_DNS_RULE = { domain_suffix: ["ts.net"], action: "route", server: "ts-dns" };

function isOwnedExternalDNSRule(rule) {
  return exactEqual(rule, EXTERNAL_DNS_RULE)
    || (Array.isArray(rule.domain_suffix)
      && rule.domain_suffix.length === 1
      && rule.domain_suffix[0] === "tailscale.com"
      && rule.action === "route"
      && typeof rule.server === "string"
      && Object.keys(rule).sort().join(",") === "action,domain_suffix,server");
}

function defaultRealDNSTag(dns, servers) {
  const matches = typeof dns.final === "string" && dns.final
    ? servers.filter((server) => server.tag === dns.final)
    : servers.slice(0, 1);
  const server = matches.length === 1 ? matches[0] : null;
  if (!server || typeof server.tag !== "string" || !server.tag.trim() || !isRealDNSServer(server)) {
    throw new Error("Sandrone sing-box Tailscale external preset with FakeIP requires a tagged real default DNS server (dns.final or the first DNS server)");
  }
  return server.tag;
}

function isRealDNSServer(server) {
  if (server.tag === "ts-dns" || server.server === "100.100.100.100" || server.type === "fakeip" || server.type === "hosts" || server.type === "tailscale") return false;
  if (server.type === "resolved") return server.accept_default_resolvers === true;
  const types = ["local", "udp", "tcp", "tls", "https", "quic", "h3", "dhcp"];
  if (server.type !== undefined && server.type !== "" && server.type !== "legacy") return types.includes(server.type);
  const address = server.address;
  if (typeof address !== "string" || !address.trim() || address.startsWith("rcode://") || address === "fakeip") return false;
  const host = address.replace(/^[a-z0-9]+:\/\//, "").split(/[/:]/)[0];
  if (host === "100.100.100.100") return false;
  const scheme = address.match(/^([a-z0-9]+):\/\//);
  return !scheme || types.includes(scheme[1]);
}

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

function ensureExactStrings(values, expectedValues) {
  const owned = new Set(expectedValues);
  const found = new Set();
  const result = [];
  for (const value of values) {
    if (!owned.has(value)) result.push(value);
    else if (!found.has(value)) { result.push(value); found.add(value); }
  }
  for (const value of expectedValues) if (!found.has(value)) result.push(value);
  return result;
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

function assertFileContext(input, presetID, managedArgs) {
  if (!isObject(input) || input.stage !== "file" || !isObject(input.file) || input.file.kind !== "sing-box") throw new Error("Sandrone sing-box " + presetID + " preset requires sing-box file-stage input");
  const requestArgs = isObject(input.request) && isObject(input.request.args) ? input.request.args : null;
  if (requestArgs && managedArgs.some((name) => Object.prototype.hasOwnProperty.call(requestArgs, name))) throw new Error("Sandrone preset arguments cannot be overridden by request args");
  if (!isObject(input.args) || input.args.preset_id !== presetID) throw new Error("Sandrone sing-box " + presetID + " preset requires preset_id=" + presetID);
}

function exactEqual(left, right) {
  if (left === right) return true;
  if (Array.isArray(left) || Array.isArray(right)) return Array.isArray(left) && Array.isArray(right) && left.length === right.length && left.every((value, index) => exactEqual(value, right[index]));
  if (!isObject(left) || !isObject(right)) return false;
  const leftKeys = Object.keys(left);
  const rightKeys = Object.keys(right);
  return leftKeys.length === rightKeys.length && leftKeys.every((key) => Object.prototype.hasOwnProperty.call(right, key) && exactEqual(left[key], right[key]));
}

function requiredObjectArray(value, message) { if (!Array.isArray(value) || value.some((item) => !isObject(item))) throw new Error(message); return value; }
function optionalObjectArray(value, message) { if (value === undefined) return []; return requiredObjectArray(value, message); }
function optionalStringArray(value, message) { if (value === undefined) return []; if (!Array.isArray(value) || value.some((item) => typeof item !== "string")) throw new Error(message); return value; }
function optionalObject(value, message) { if (value === undefined) return {}; if (!isObject(value)) throw new Error(message); return value; }
function isObject(value) { return typeof value === "object" && value !== null && !Array.isArray(value); }
