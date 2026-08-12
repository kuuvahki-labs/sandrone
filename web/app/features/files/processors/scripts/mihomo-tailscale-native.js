/* eslint-disable unused-imports/no-unused-vars */

function main(input, api) {
  const document = api.yaml.parse(input.file.content);
  if (!isObject(document)) {
    throw new Error("Sandrone Mihomo Tailscale native preset requires a YAML object");
  }
  if (!Array.isArray(document.proxies) || document.proxies.some((proxy) => !isObject(proxy))) {
    throw new Error("Sandrone Mihomo Tailscale native preset requires proxies to be an array of objects");
  }
  if (!Array.isArray(document.rules) || document.rules.some((rule) => typeof rule !== "string")) {
    throw new Error("Sandrone Mihomo Tailscale native preset requires rules to be an array of strings");
  }
  if (document.dns !== undefined && !isObject(document.dns)) {
    throw new Error("Sandrone Mihomo Tailscale native preset requires dns to be an object");
  }
  if (document.tun !== undefined && !isObject(document.tun)) {
    throw new Error("Sandrone Mihomo Tailscale native preset requires tun to be an object");
  }

  const dns = document.dns || {};
  const fakeIPFilter = dns["fake-ip-filter"] === undefined ? [] : dns["fake-ip-filter"];
  if (!isStringArray(fakeIPFilter)) {
    throw new Error("Sandrone Mihomo Tailscale native preset requires dns.fake-ip-filter to be an array of strings");
  }
  if (dns["nameserver-policy"] !== undefined && !isObject(dns["nameserver-policy"])) {
    throw new Error("Sandrone Mihomo Tailscale native preset requires dns.nameserver-policy to be an object");
  }

  const tun = document.tun || {};
  const routeExclusions = tun["route-exclude-address"] === undefined
    ? []
    : tun["route-exclude-address"];
  if (!isStringArray(routeExclusions)) {
    throw new Error("Sandrone Mihomo Tailscale native preset requires tun.route-exclude-address to be an array of strings");
  }

  for (const proxy of document.proxies) {
    if (proxy.name === "TAILSCALE" && !isExactTailscaleProxy(proxy)) {
      throw new Error("Sandrone Mihomo Tailscale native preset found incompatible proxy named TAILSCALE");
    }
  }

  const anchorIndex = firstAnchorIndex(document.rules, [
    "RULE-SET,private,",
    "GEOIP,CN,",
    "MATCH,",
  ]);
  if (anchorIndex < 0) {
    throw new Error("Sandrone preset tailscale-native cannot find a safe mihomo rule anchor");
  }

  const proxies = [];
  let hasTailscaleProxy = false;
  for (const proxy of document.proxies) {
    if (proxy.name !== "TAILSCALE") {
      proxies.push(proxy);
    } else if (!hasTailscaleProxy) {
      proxies.push(proxy);
      hasTailscaleProxy = true;
    }
  }
  if (!hasTailscaleProxy) {
    proxies.push({
      name: "TAILSCALE",
      type: "tailscale",
      ephemeral: false,
      udp: true,
      "accept-routes": false,
    });
  }

  const rules = document.rules.filter((rule) => !TAILSCALE_RULES.includes(rule));
  const insertionIndex = document.rules
    .slice(0, anchorIndex)
    .filter((rule) => !TAILSCALE_RULES.includes(rule))
    .length;
  rules.splice(insertionIndex, 0, ...TAILSCALE_RULES);

  const updated = {
    ...document,
    proxies,
    rules,
    dns: {
      ...dns,
      "fake-ip-filter": ensureOneExactValue(fakeIPFilter, "+.ts.net"),
      "nameserver-policy": {
        ...(dns["nameserver-policy"] || {}),
        "+.ts.net": "100.100.100.100",
      },
    },
    tun: {
      ...tun,
      "route-exclude-address": routeExclusions.filter((value) => (
        value !== "100.64.0.0/10" && value !== "fd7a:115c:a1e0::/48"
      )),
    },
  };

  const content = api.yaml.stringify(updated);
  input.file.content = content;
  return input;
}

const TAILSCALE_RULES = [
  "DOMAIN-SUFFIX,ts.net,TAILSCALE",
  "IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
  "IP-CIDR6,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve",
];

function isExactTailscaleProxy(proxy) {
  const keys = Object.keys(proxy).sort();
  const expectedKeys = ["accept-routes", "ephemeral", "name", "type", "udp"];
  return keys.length === expectedKeys.length
    && keys.every((key, index) => key === expectedKeys[index])
    && proxy.name === "TAILSCALE"
    && proxy.type === "tailscale"
    && proxy.ephemeral === false
    && proxy.udp === true
    && proxy["accept-routes"] === false;
}

function firstAnchorIndex(rules, prefixes) {
  for (const prefix of prefixes) {
    const index = rules.findIndex((rule) => rule.startsWith(prefix));
    if (index >= 0) return index;
  }
  return -1;
}

function isStringArray(value) {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function ensureOneExactValue(values, expected) {
  const result = [];
  let found = false;
  for (const value of values) {
    if (value !== expected) {
      result.push(value);
    } else if (!found) {
      result.push(value);
      found = true;
    }
  }
  if (!found) result.push(expected);
  return result;
}

function isObject(value) {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
