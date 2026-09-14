/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - preset_id: stable preset identifier.
// - server: optional tagged real resolver; otherwise dns.final or the first server is used.
function main(input, api) {
  const managedArgs = ["preset_id", "server"];
  assertFileContext(input, "fakeip-ruleset-geodata", managedArgs);
  const explicitServer = stringArgument(input, "server");

  const document = api.json.parse(input.file.content);
  if (!isObject(document)) {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset requires a JSON object");
  }
  const dns = requiredObject(
    document.dns,
    "Sandrone sing-box fakeip-ruleset-geodata preset requires dns to be an object",
  );
  const dnsServers = requiredObjectArray(
    dns.servers,
    "Sandrone sing-box fakeip-ruleset-geodata preset requires dns.servers to be an array of objects",
  );
  const dnsRules = requiredObjectArray(
    dns.rules,
    "Sandrone sing-box fakeip-ruleset-geodata preset requires dns.rules to be an array of objects",
  );
  const route = requiredObject(
    document.route,
    "Sandrone sing-box fakeip-ruleset-geodata preset requires route to be an object",
  );
  const ruleSets = optionalObjectArray(
    route.rule_set,
    "Sandrone sing-box fakeip-ruleset-geodata preset requires route.rule_set to be an array of objects",
  );

  const fakeTags = new Set(
    dnsServers
      .filter((server) => server.type === "fakeip" && validTag(server.tag))
      .map((server) => server.tag),
  );
  if (fakeTags.size === 0) {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset requires a tagged FakeIP DNS server");
  }
  if (!dnsRules.some((rule) => typeof rule.server === "string" && fakeTags.has(rule.server))) {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset requires a DNS rule routed to FakeIP");
  }

  const selectedServer = selectRealDNSTag(explicitServer, dns, dnsServers);
  const selectedHTTPClient = selectDefaultHTTPClient(document, route);
  const managedRuleSetIndices = [];
  ruleSets.forEach((ruleSet, index) => {
    if (containsRuleSet(ruleSet.tag, RULE_SET_TAG)) managedRuleSetIndices.push(index);
  });
  if (managedRuleSetIndices.length > 1) {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset found duplicate route rule-set tag " + RULE_SET_TAG);
  }
  if (managedRuleSetIndices.length === 1 && !isCompatibleManagedRuleSet(ruleSets[managedRuleSetIndices[0]])) {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset found incompatible route rule-set tag " + RULE_SET_TAG);
  }

  const ownedDNSIndices = [];
  dnsRules.forEach((rule, index) => {
    if (containsRuleSet(rule.rule_set, RULE_SET_TAG)) {
      if (!isCompatibleManagedDNSRule(rule)) {
        throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset found incompatible DNS rule for " + RULE_SET_TAG);
      }
      ownedDNSIndices.push(index);
    }
  });

  const managedRuleSet = {
    type: "remote",
    tag: RULE_SET_TAG,
    format: "binary",
    url: RULE_SET_URL,
    http_client: selectedHTTPClient,
    update_interval: "1d",
  };
  const updatedRuleSets = [...ruleSets];
  if (managedRuleSetIndices.length === 1) updatedRuleSets[managedRuleSetIndices[0]] = managedRuleSet;
  else updatedRuleSets.push(managedRuleSet);

  const ownedDNSIndexSet = new Set(ownedDNSIndices);
  const retainedDNSRules = dnsRules.filter((_, index) => !ownedDNSIndexSet.has(index));
  const retainedFakeIndex = retainedDNSRules.findIndex((rule) => (
    typeof rule.server === "string" && fakeTags.has(rule.server)
  ));
  if (retainedFakeIndex < 0) {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset requires a DNS rule routed to FakeIP");
  }
  const managedDNSRule = { rule_set: [RULE_SET_TAG], action: "route", server: selectedServer };
  retainedDNSRules.splice(retainedFakeIndex, 0, managedDNSRule);

  const updated = {
    ...document,
    dns: { ...dns, rules: retainedDNSRules },
    route: { ...route, rule_set: updatedRuleSets },
  };
  input.file.content = api.json.stringify(updated);
  return input;
}

const RULE_SET_TAG = "sandrone-fakeip-ruleset-geodata";
const RULE_SET_URL = "https://cdn.jsdelivr.net/gh/DustinWin/ruleset_geodata@sing-box-ruleset/fakeip-filter.srs";

function isCompatibleManagedRuleSet(ruleSet) {
  return Object.keys(ruleSet).sort().join(",") === "format,http_client,tag,type,update_interval,url"
    && ruleSet.type === "remote"
    && ruleSet.tag === RULE_SET_TAG
    && ruleSet.format === "binary"
    && ruleSet.url === RULE_SET_URL
    && validTag(ruleSet.http_client)
    && ruleSet.update_interval === "1d";
}

function isCompatibleManagedDNSRule(rule) {
  return Object.keys(rule).sort().join(",") === "action,rule_set,server"
    && rule.action === "route"
    && typeof rule.server === "string"
    && Array.isArray(rule.rule_set)
    && rule.rule_set.length === 1
    && rule.rule_set[0] === RULE_SET_TAG;
}

function selectRealDNSTag(explicitServer, dns, servers) {
  let matches;
  if (explicitServer) matches = servers.filter((server) => server.tag === explicitServer);
  else if (typeof dns.final === "string" && dns.final) matches = servers.filter((server) => server.tag === dns.final);
  else matches = servers.slice(0, 1);
  const server = matches.length === 1 ? matches[0] : null;
  if (!server || !validTag(server.tag) || !isRealDNSServer(server)) {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset requires a tagged real DNS resolver");
  }
  return server.tag;
}

function selectDefaultHTTPClient(document, route) {
  const clients = optionalObjectArray(
    document.http_clients,
    "Sandrone sing-box fakeip-ruleset-geodata preset requires http_clients to be an array of objects",
  );
  let defaultTag = "";
  if (validTag(route.default_http_client)) defaultTag = route.default_http_client;
  else if (clients.length === 1 && validTag(clients[0].tag)) defaultTag = clients[0].tag;
  const matches = clients.filter((client) => client.tag === defaultTag);
  if (!validTag(defaultTag) || matches.length !== 1) {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset requires one configured default HTTP client");
  }
  return defaultTag;
}

function isRealDNSServer(server) {
  if (server.tag === "ts-dns" || server.server === "100.100.100.100"
    || server.type === "fakeip" || server.type === "hosts" || server.type === "tailscale") return false;
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

function containsRuleSet(value, tag) {
  return Array.isArray(value) ? value.includes(tag) : value === tag;
}

function validTag(value) {
  return typeof value === "string" && value.trim() !== "";
}

function stringArgument(input, name) {
  const value = input.args[name];
  if (typeof value !== "string") {
    throw new Error("Sandrone sing-box fakeip-ruleset-geodata preset requires string arg " + name);
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

function requiredObject(value, message) {
  if (!isObject(value)) throw new Error(message);
  return value;
}

function requiredObjectArray(value, message) {
  if (!Array.isArray(value) || value.some((item) => !isObject(item))) throw new Error(message);
  return value;
}

function optionalObjectArray(value, message) {
  if (value === undefined) return [];
  return requiredObjectArray(value, message);
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
