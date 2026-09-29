/*
 * Sandrone Mihomo / sing-box Tailscale 原生接管示例
 *
 * 这是一个可登记的 file-stage script，不属于客户端预设目录。复制或登记后，
 * 将它加入对应文件的 processor chain，并按需填写：
 *   auth_key       可选，留空时由目标客户端提供交互式登录
 *
 * Mihomo 和 sing-box 的 Tailscale 原生结构不同，本脚本通过客户端 adapter
 * 分别生成对应的 proxy/endpoint、MagicDNS 和路由规则。Shadowrocket 的原生
 * TAILSCALE policy 仍由它自己的预设负责。
 * 使用前移除该文件的 Tailscale 共存 processor，并把本脚本放在处理链末尾。
 */

var SCRIPT_NAME = "tailscale-native.js";

var ADAPTERS = {
    mihomo: {apply: applyMihomo},
    "sing-box": {apply: applySingBox}
};

function main(input, api) {
    requireFile(input);
    var adapter = ADAPTERS[input.file.kind];
    if (!adapter) throw new Error(SCRIPT_NAME + " does not support file kind: " + input.file.kind);

    var args = readArgs(input.args || {});
    var content = adapter.apply(input.file.content, args, api);
    if (content !== undefined) input.file.content = content;
    return input;
}

function requireFile(input) {
    if (!input || input.stage !== "file" || !input.file ||
        typeof input.file.content !== "string") {
        throw new Error(SCRIPT_NAME + " requires file-stage text input");
    }
}

function readArgs(args) {
    var value = args.auth_key;
    if (value === undefined || value === null || value === "") return {authKey: ""};
    if (typeof value !== "string") throw new Error(SCRIPT_NAME + " auth_key must be a string");
    return {authKey: value};
}

/* Mihomo adapter */

function applyMihomo(content, args, api) {
    var document = api.yaml.parse(content);
    if (!mihomoIsObject(document)) {
        throw new Error(SCRIPT_NAME + " Mihomo document requires a YAML object");
    }
    if (!Array.isArray(document.proxies) || document.proxies.some(function(proxy) { return !mihomoIsObject(proxy); })) {
        throw new Error(SCRIPT_NAME + " Mihomo document requires proxies to be an array of objects");
    }
    if (!Array.isArray(document.rules) || document.rules.some(function(rule) { return typeof rule !== "string"; })) {
        throw new Error(SCRIPT_NAME + " Mihomo document requires rules to be an array of strings");
    }
    if (document.dns !== undefined && !mihomoIsObject(document.dns)) {
        throw new Error(SCRIPT_NAME + " Mihomo document requires dns to be an object");
    }
    if (document.tun !== undefined && !mihomoIsObject(document.tun)) {
        throw new Error(SCRIPT_NAME + " Mihomo document requires tun to be an object");
    }

    var dns = document.dns || {};
    var fakeIPFilter = dns["fake-ip-filter"] === undefined ? [] : dns["fake-ip-filter"];
    if (!mihomoIsStringArray(fakeIPFilter)) {
        throw new Error(SCRIPT_NAME + " Mihomo dns.fake-ip-filter must be an array of strings");
    }
    if (dns["nameserver-policy"] !== undefined && !mihomoIsObject(dns["nameserver-policy"])) {
        throw new Error(SCRIPT_NAME + " Mihomo dns.nameserver-policy must be an object");
    }

    var tun = document.tun || {};
    var routeExclusions = tun["route-exclude-address"] === undefined
        ? []
        : tun["route-exclude-address"];
    if (!mihomoIsStringArray(routeExclusions)) {
        throw new Error(SCRIPT_NAME + " Mihomo tun.route-exclude-address must be an array of strings");
    }

    document.proxies.forEach(function(proxy) {
        if (proxy.name === "TAILSCALE" && !mihomoIsExactTailscaleProxy(proxy)) {
            throw new Error(SCRIPT_NAME + " Mihomo found incompatible proxy named TAILSCALE");
        }
    });

    var anchorIndex = mihomoFirstAnchorIndex(document.rules, [
        "RULE-SET,private,",
        "GEOIP,CN,",
        "MATCH,"
    ]);
    if (anchorIndex < 0) {
        throw new Error(SCRIPT_NAME + " Mihomo cannot find a safe rule anchor");
    }
    if (document.rules.some(function(rule) { return MIHOMO_EXTERNAL_ROUTE_RULES.indexOf(rule) >= 0; })) {
        throw new Error(SCRIPT_NAME + " Mihomo requires removing the Tailscale coexistence processor first");
    }

    var proxies = [];
    var hasTailscaleProxy = false;
    document.proxies.forEach(function(proxy) {
        if (proxy.name !== "TAILSCALE") {
            proxies.push(proxy);
        } else if (!hasTailscaleProxy) {
            proxies.push(mihomoTailscaleProxy(args.authKey));
            hasTailscaleProxy = true;
        }
    });
    if (!hasTailscaleProxy) proxies.push(mihomoTailscaleProxy(args.authKey));

    var rules = document.rules.filter(function(rule) { return MIHOMO_TAILSCALE_RULES.indexOf(rule) < 0; });
    var insertionIndex = document.rules
        .slice(0, anchorIndex)
        .filter(function(rule) { return MIHOMO_TAILSCALE_RULES.indexOf(rule) < 0; })
        .length;
    rules.splice.apply(rules, [insertionIndex, 0].concat(MIHOMO_TAILSCALE_RULES));

    var updated = {
        ...document,
        proxies: proxies,
        rules: rules,
        dns: {
            ...dns,
            "fake-ip-filter": mihomoEnsureOneExactValue(fakeIPFilter, "+.ts.net"),
            "nameserver-policy": {
                ...(dns["nameserver-policy"] || {}),
                "+.ts.net": "100.100.100.100"
            }
        },
        tun: {
            ...tun,
            "route-exclude-address": routeExclusions.filter(function(value) {
                return MIHOMO_TAILSCALE_RANGES.indexOf(value) < 0;
            })
        }
    };

    return api.yaml.stringify(updated);
}

var MIHOMO_TAILSCALE_RANGES = ["100.64.0.0/10", "fd7a:115c:a1e0::/48"];
var MIHOMO_TAILSCALE_RULES = [
    "DOMAIN-SUFFIX,ts.net,TAILSCALE",
    "IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
    "IP-CIDR6,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve"
];
var MIHOMO_EXTERNAL_ROUTE_RULES = [
    "IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
    "IP-CIDR6,fd7a:115c:a1e0::/48,DIRECT,no-resolve"
];

function mihomoIsExactTailscaleProxy(proxy) {
    var keys = Object.keys(proxy).sort();
    var baseKeys = ["accept-routes", "ephemeral", "name", "type", "udp"];
    var authKeys = ["accept-routes", "auth-key", "ephemeral", "name", "type", "udp"];
    var expectedKeys = Object.hasOwn(proxy, "auth-key") ? authKeys : baseKeys;
    return keys.length === expectedKeys.length
        && keys.every(function(key, index) { return key === expectedKeys[index]; })
        && proxy.name === "TAILSCALE"
        && proxy.type === "tailscale"
        && proxy.ephemeral === false
        && proxy.udp === true
        && proxy["accept-routes"] === false
        && (!Object.hasOwn(proxy, "auth-key") || typeof proxy["auth-key"] === "string");
}

function mihomoTailscaleProxy(authKey) {
    return {
        name: "TAILSCALE",
        type: "tailscale",
        ...(authKey ? {"auth-key": authKey} : {}),
        ephemeral: false,
        udp: true,
        "accept-routes": false
    };
}

function mihomoFirstAnchorIndex(rules, prefixes) {
    for (var index = 0; index < prefixes.length; index += 1) {
        var found = rules.findIndex(function(rule) { return rule.indexOf(prefixes[index]) === 0; });
        if (found >= 0) return found;
    }
    return -1;
}

function mihomoIsStringArray(value) {
    return Array.isArray(value) && value.every(function(item) { return typeof item === "string"; });
}

function mihomoEnsureOneExactValue(values, expected) {
    var result = [];
    var found = false;
    values.forEach(function(value) {
        if (value !== expected) {
            result.push(value);
        } else if (!found) {
            result.push(value);
            found = true;
        }
    });
    if (!found) result.push(expected);
    return result;
}

function mihomoIsObject(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}

/* sing-box adapter */

function applySingBox(content, args, api) {
    var document = api.json.parse(content);
    if (!singBoxIsObject(document)) {
        throw new Error(SCRIPT_NAME + " sing-box document requires a JSON object");
    }

    var inbounds = singBoxRequiredObjectArray(
        document.inbounds,
        SCRIPT_NAME + " sing-box document requires inbounds to be an array of objects"
    );
    var tunIndex = singBoxSelectManagedTunIndex(inbounds);
    var selectedTun = inbounds[tunIndex];
    var routeExclusions = singBoxOptionalStringArray(
        selectedTun.route_exclude_address,
        SCRIPT_NAME + " sing-box TUN route_exclude_address must be an array of strings"
    );
    var endpoints = singBoxOptionalObjectArray(
        document.endpoints,
        SCRIPT_NAME + " sing-box endpoints must be an array of objects"
    );
    var outbounds = singBoxOptionalObjectArray(
        document.outbounds,
        SCRIPT_NAME + " sing-box outbounds must be an array of objects"
    );
    var dns = singBoxOptionalObject(
        document.dns,
        SCRIPT_NAME + " sing-box dns must be an object"
    );
    var dnsServers = singBoxOptionalObjectArray(
        dns.servers,
        SCRIPT_NAME + " sing-box dns.servers must be an array of objects"
    );
    var dnsRules = singBoxOptionalObjectArray(
        dns.rules,
        SCRIPT_NAME + " sing-box dns.rules must be an array of objects"
    );
    var route = singBoxRequiredObject(
        document.route,
        SCRIPT_NAME + " sing-box document requires a route object"
    );
    var routeRules = singBoxRequiredObjectArray(
        route.rules,
        SCRIPT_NAME + " sing-box route.rules must be an array of objects"
    );
    if (routeRules.some(function(rule) {
        return SING_BOX_EXTERNAL_ROUTE_RULES.some(function(externalRule) {
            return singBoxExactEqual(rule, externalRule);
        });
    })) {
        throw new Error(SCRIPT_NAME + " sing-box requires removing the Tailscale coexistence processor first");
    }

    endpoints.forEach(function(endpoint) {
        if (endpoint.tag === "ts-ep" && !singBoxIsCompatibleNativeEndpoint(endpoint)) {
            throw new Error(SCRIPT_NAME + " sing-box found incompatible endpoint tag ts-ep");
        }
    });
    outbounds.forEach(function(outbound) {
        if (outbound.tag === "ts-ep") {
            throw new Error(SCRIPT_NAME + " sing-box found incompatible outbound tag ts-ep");
        }
    });
    dnsServers.forEach(function(server) {
        if (server.tag === "ts-dns" && !singBoxExactEqual(server, SING_BOX_NATIVE_DNS_SERVER)) {
            throw new Error(SCRIPT_NAME + " sing-box found incompatible DNS server tag ts-dns");
        }
    });

    var anchorIndex = singBoxFirstAnchorIndex(routeRules);
    if (anchorIndex < 0) throw new Error(SCRIPT_NAME + " sing-box cannot find a safe rule anchor");
    var updatedRouteRules = routeRules.filter(function(rule) {
        return !singBoxExactEqual(rule, SING_BOX_NATIVE_ROUTE_RULE);
    });
    var routeInsertionIndex = routeRules
        .slice(0, anchorIndex)
        .filter(function(rule) { return !singBoxExactEqual(rule, SING_BOX_NATIVE_ROUTE_RULE); })
        .length;
    updatedRouteRules.splice(routeInsertionIndex, 0, SING_BOX_NATIVE_ROUTE_RULE);

    var fakeTags = new Set(dnsServers
        .filter(function(server) { return server.type === "fakeip" && typeof server.tag === "string"; })
        .map(function(server) { return server.tag; }));
    var retainedDNSRules = dnsRules.filter(function(rule) {
        return !singBoxExactEqual(rule, SING_BOX_NATIVE_DNS_RULE);
    });
    var fakeRuleIndex = retainedDNSRules.findIndex(function(rule) {
        return typeof rule.server === "string" && fakeTags.has(rule.server);
    });
    retainedDNSRules.splice(fakeRuleIndex < 0 ? retainedDNSRules.length : fakeRuleIndex, 0, SING_BOX_NATIVE_DNS_RULE);

    var updatedInbounds = inbounds.slice();
    updatedInbounds[tunIndex] = {
        ...selectedTun,
        route_exclude_address: routeExclusions.filter(function(value) {
            return SING_BOX_TAILSCALE_RANGES.indexOf(value) < 0;
        })
    };
    var updated = {
        ...document,
        endpoints: singBoxEnsureOneTaggedObject(endpoints, singBoxNativeEndpoint(args.authKey)),
        dns: {
            ...dns,
            servers: singBoxEnsureOneExactObject(dnsServers, SING_BOX_NATIVE_DNS_SERVER),
            rules: retainedDNSRules
        },
        inbounds: updatedInbounds,
        route: {...route, rules: updatedRouteRules}
    };
    return api.json.stringify(updated);
}

var SING_BOX_TAILSCALE_RANGES = ["100.64.0.0/10", "fd7a:115c:a1e0::/48"];
var SING_BOX_NATIVE_ENDPOINT = {
    type: "tailscale",
    tag: "ts-ep",
    ephemeral: false,
    accept_routes: false
};
var SING_BOX_NATIVE_DNS_SERVER = {
    type: "tailscale",
    tag: "ts-dns",
    endpoint: "ts-ep",
    accept_default_resolvers: false
};
var SING_BOX_NATIVE_DNS_RULE = {preferred_by: "ts-dns", action: "route", server: "ts-dns"};
var SING_BOX_NATIVE_ROUTE_RULE = {preferred_by: ["ts-ep"], action: "route", outbound: "ts-ep"};
var SING_BOX_EXTERNAL_ROUTE_RULES = [
    {domain_suffix: ["tailscale.com"], outbound: "direct"},
    {ip_cidr: SING_BOX_TAILSCALE_RANGES, outbound: "direct"}
];

function singBoxNativeEndpoint(authKey) {
    return {
        ...SING_BOX_NATIVE_ENDPOINT,
        ...(authKey ? {auth_key: authKey} : {})
    };
}

function singBoxIsCompatibleNativeEndpoint(endpoint) {
    var expectedKeys = Object.hasOwn(endpoint, "auth_key")
        ? ["accept_routes", "auth_key", "ephemeral", "tag", "type"]
        : ["accept_routes", "ephemeral", "tag", "type"];
    var keys = Object.keys(endpoint).sort();
    return keys.length === expectedKeys.length
        && keys.every(function(key, index) { return key === expectedKeys[index]; })
        && endpoint.type === SING_BOX_NATIVE_ENDPOINT.type
        && endpoint.tag === SING_BOX_NATIVE_ENDPOINT.tag
        && endpoint.ephemeral === SING_BOX_NATIVE_ENDPOINT.ephemeral
        && endpoint.accept_routes === SING_BOX_NATIVE_ENDPOINT.accept_routes
        && (!Object.hasOwn(endpoint, "auth_key") || typeof endpoint.auth_key === "string");
}

function singBoxFirstAnchorIndex(rules) {
    var privateRuleSet = rules.findIndex(function(rule) {
        return singBoxContainsPrivateRuleSet(rule.rule_set);
    });
    if (privateRuleSet >= 0) return privateRuleSet;
    var privateIP = rules.findIndex(function(rule) { return rule.ip_is_private === true; });
    if (privateIP >= 0) return privateIP;
    return rules.findIndex(singBoxIsMatchAllFinalRule);
}

function singBoxContainsPrivateRuleSet(value) {
    return Array.isArray(value) ? value.indexOf("private") >= 0 : value === "private";
}

function singBoxIsMatchAllFinalRule(rule) {
    if (typeof rule.outbound !== "string") return false;
    return Object.keys(rule).every(function(key) {
        return key === "outbound" || key === "action" && rule.action === "route";
    });
}

function singBoxSelectManagedTunIndex(inbounds) {
    var tagged = [];
    var tun = [];
    inbounds.forEach(function(inbound, index) {
        if (inbound.tag === "tun-in") tagged.push(index);
        if (inbound.type === "tun") tun.push(index);
    });
    if (tagged.length > 1 || tun.length > 1) {
        throw new Error(SCRIPT_NAME + " sing-box found ambiguous TUN inbounds");
    }
    if (tagged.length !== 1 || inbounds[tagged[0]].type !== "tun") {
        throw new Error(SCRIPT_NAME + " sing-box requires the tun-in TUN inbound");
    }
    if (tun[0] !== tagged[0]) {
        throw new Error(SCRIPT_NAME + " sing-box found an unmanaged TUN inbound");
    }
    return tagged[0];
}

function singBoxEnsureOneExactObject(values, expected) {
    var result = [];
    var found = false;
    values.forEach(function(value) {
        if (!singBoxExactEqual(value, expected)) {
            result.push(value);
        } else if (!found) {
            result.push(expected);
            found = true;
        }
    });
    if (!found) result.push(expected);
    return result;
}

function singBoxEnsureOneTaggedObject(values, expected) {
    var result = [];
    var found = false;
    values.forEach(function(value) {
        if (value.tag !== expected.tag) {
            result.push(value);
        } else if (!found) {
            result.push(expected);
            found = true;
        }
    });
    if (!found) result.push(expected);
    return result;
}

function singBoxExactEqual(left, right) {
    if (left === right) return true;
    if (Array.isArray(left) || Array.isArray(right)) {
        return Array.isArray(left) && Array.isArray(right)
            && left.length === right.length
            && left.every(function(value, index) { return singBoxExactEqual(value, right[index]); });
    }
    if (!singBoxIsObject(left) || !singBoxIsObject(right)) return false;
    var leftKeys = Object.keys(left);
    var rightKeys = Object.keys(right);
    return leftKeys.length === rightKeys.length && leftKeys.every(function(key) {
        return Object.prototype.hasOwnProperty.call(right, key) && singBoxExactEqual(left[key], right[key]);
    });
}

function singBoxRequiredObject(value, message) {
    if (!singBoxIsObject(value)) throw new Error(message);
    return value;
}

function singBoxRequiredObjectArray(value, message) {
    if (!Array.isArray(value) || value.some(function(item) { return !singBoxIsObject(item); })) {
        throw new Error(message);
    }
    return value;
}

function singBoxOptionalObjectArray(value, message) {
    if (value === undefined) return [];
    return singBoxRequiredObjectArray(value, message);
}

function singBoxOptionalStringArray(value, message) {
    if (value === undefined) return [];
    if (!Array.isArray(value) || value.some(function(item) { return typeof item !== "string"; })) {
        throw new Error(message);
    }
    return value;
}

function singBoxOptionalObject(value, message) {
    if (value === undefined) return {};
    return singBoxRequiredObject(value, message);
}

function singBoxIsObject(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
