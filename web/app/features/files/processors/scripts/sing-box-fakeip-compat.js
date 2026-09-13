/* eslint-disable unused-imports/no-unused-vars */

// Parameters:
// - preset_id: stable preset identifier.
// - domain/domain_suffix/domain_regex: one inline headless compatibility rule.
// - server: optional tagged real resolver; otherwise dns.final or the first server is used.
function main(input, api) {
  const managedArgs = ["preset_id", "domain", "domain_suffix", "domain_regex", "server"];
  assertFileContext(input, "fakeip-compat", managedArgs);
  const domain = uniqueStrings(stringArrayArgument(input, "domain"));
  const domainSuffix = uniqueStrings(stringArrayArgument(input, "domain_suffix"));
  const domainRegex = uniqueStrings(stringArrayArgument(input, "domain_regex"));
  const explicitServer = stringArgument(input, "server");
  if (domain.length + domainSuffix.length + domainRegex.length === 0) {
    throw new Error("Sandrone sing-box fakeip-compat preset requires at least one compatibility domain");
  }

  const document = api.json.parse(input.file.content);
  if (!isObject(document)) {
    throw new Error("Sandrone sing-box fakeip-compat preset requires a JSON object");
  }
  const dns = requiredObject(document.dns, "Sandrone sing-box fakeip-compat preset requires dns to be an object");
  const dnsServers = requiredObjectArray(
    dns.servers,
    "Sandrone sing-box fakeip-compat preset requires dns.servers to be an array of objects",
  );
  const dnsRules = requiredObjectArray(
    dns.rules,
    "Sandrone sing-box fakeip-compat preset requires dns.rules to be an array of objects",
  );
  const route = requiredObject(document.route, "Sandrone sing-box fakeip-compat preset requires route to be an object");
  const ruleSets = optionalObjectArray(
    route.rule_set,
    "Sandrone sing-box fakeip-compat preset requires route.rule_set to be an array of objects",
  );

  const fakeTags = new Set(dnsServers.filter((server) => server.type === "fakeip" && validTag(server.tag)).map((server) => server.tag));
  if (fakeTags.size === 0) {
    throw new Error("Sandrone sing-box fakeip-compat preset requires a tagged FakeIP DNS server");
  }
  const fakeRuleIndex = dnsRules.findIndex((rule) => typeof rule.server === "string" && fakeTags.has(rule.server));
  if (fakeRuleIndex < 0) {
    throw new Error("Sandrone sing-box fakeip-compat preset requires a DNS rule routed to FakeIP");
  }

  const selectedServer = selectRealDNSTag(explicitServer, dns, dnsServers);
  const managedRuleSetIndices = [];
  ruleSets.forEach((ruleSet, index) => {
    if (ruleSet.tag === RULE_SET_TAG) managedRuleSetIndices.push(index);
  });
  if (managedRuleSetIndices.length > 1) {
    throw new Error("Sandrone sing-box fakeip-compat preset found duplicate route rule-set tag " + RULE_SET_TAG);
  }
  if (managedRuleSetIndices.length === 1 && !isCompatibleManagedRuleSet(ruleSets[managedRuleSetIndices[0]])) {
    throw new Error("Sandrone sing-box fakeip-compat preset found incompatible route rule-set tag " + RULE_SET_TAG);
  }

  const ownedDNSIndices = [];
  dnsRules.forEach((rule, index) => {
    if (containsRuleSet(rule.rule_set, RULE_SET_TAG)) {
      if (!isCompatibleManagedDNSRule(rule)) {
        throw new Error("Sandrone sing-box fakeip-compat preset found incompatible DNS rule for " + RULE_SET_TAG);
      }
      ownedDNSIndices.push(index);
    }
  });

  const inlineRule = {
    ...(domain.length ? { domain } : {}),
    ...(domainSuffix.length ? { domain_suffix: domainSuffix } : {}),
    ...(domainRegex.length ? { domain_regex: domainRegex } : {}),
  };
  const managedRuleSet = { type: "inline", tag: RULE_SET_TAG, rules: [inlineRule] };
  const updatedRuleSets = [...ruleSets];
  if (managedRuleSetIndices.length === 1) updatedRuleSets[managedRuleSetIndices[0]] = managedRuleSet;
  else updatedRuleSets.push(managedRuleSet);

  const ownedDNSIndexSet = new Set(ownedDNSIndices);
  const retainedDNSRules = dnsRules.filter((_, index) => !ownedDNSIndexSet.has(index));
  const retainedFakeIndex = retainedDNSRules.findIndex((rule) => (
    typeof rule.server === "string" && fakeTags.has(rule.server)
  ));
  if (retainedFakeIndex < 0) {
    throw new Error("Sandrone sing-box fakeip-compat preset requires a DNS rule routed to FakeIP");
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

const RULE_SET_TAG = "sandrone-fakeip-compat";

function isCompatibleManagedRuleSet(ruleSet) {
  if (Object.keys(ruleSet).sort().join(",") !== "rules,tag,type"
    || ruleSet.type !== "inline"
    || ruleSet.tag !== RULE_SET_TAG
    || !Array.isArray(ruleSet.rules)
    || ruleSet.rules.length !== 1
    || !isObject(ruleSet.rules[0])) return false;
  const rule = ruleSet.rules[0];
  const allowed = new Set(["domain", "domain_suffix", "domain_regex"]);
  return Object.keys(rule).every((key) => allowed.has(key))
    && Object.values(rule).every((value) => (
      Array.isArray(value) && value.every((item) => typeof item === "string")
    ));
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
    throw new Error("Sandrone sing-box fakeip-compat preset requires a tagged real DNS resolver");
  }
  return server.tag;
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

function uniqueStrings(values) {
  return [...new Set(values)];
}

function validTag(value) {
  return typeof value === "string" && value.trim() !== "";
}

function stringArgument(input, name) {
  const value = input.args[name];
  if (typeof value !== "string") throw new Error("Sandrone sing-box fakeip-compat preset requires string arg " + name);
  return value;
}

function stringArrayArgument(input, name) {
  const value = input.args[name];
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string" || item === "")) {
    throw new Error("Sandrone sing-box fakeip-compat preset requires string array arg " + name);
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
