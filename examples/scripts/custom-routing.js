/*
 * custom-routing.js
 *
 * 通过 params.args.rules 提供一份抽象规则，同时生成：
 * - Mihomo rules
 * - sing-box route.rules
 * - Shadowrocket [Rule]
 *
 * params.args（Web 参数每行 key=value）：
 *   rules=[{"type":"domain_suffix","value":"example.com","route":"direct"}]
 *   routes={"ai":"AI"}                       可选，自定义路由映射
 *
 * rules 必须是数组或 JSON 数组字符串。支持 domain、domain_suffix、
 * domain_keyword、ip_cidr、port。route 可用 direct、proxy、reject，
 * 也可引用 routes 中的名称。
 *
 * routes 的值可以是三端共用的策略名，也可以按客户端分别指定：
 *   {"ai":{"mihomo":"AI","sing-box":"人工智能","shadowrocket":"AI服务"}}
 *
 * proxy 自动使用当前配置的最终策略。脚本不创建自定义策略所引用的代理组。
 * 新规则会放在已有普通路由规则之前；sing-box 已有的前置动作和 Clash 模式
 * 规则保持优先。
 */

var SCRIPT_NAME = "custom-routing.js";

var ADAPTERS = {
    mihomo: {apply: applyMihomo},
    "sing-box": {apply: applySingBox},
    shadowrocket: {apply: applyShadowrocket}
};

function main(input, api) {
    requireFile(input);
    var adapter = ADAPTERS[input.file.kind];
    if (!adapter) {
        throw new Error(SCRIPT_NAME + " does not support file kind: " + input.file.kind);
    }

    var args = readArgs(input.args || {}, api);
    var content = applyOperation(input.file.content, args, api, adapter);
    if (content !== undefined) input.file.content = content;
    return input;
}

function readArgs(args, api) {
    var rules = readJSONArray(args.rules, "rules", api);
    var routes = readJSONObject(args.routes, "routes", api, {});
    validateRules(rules);
    validateRoutes(routes);
    return {rules: rules, routes: routes};
}

function applyOperation(content, args, api, adapter) {
    return adapter.apply(content, args, api);
}

/* Mihomo */

function applyMihomo(content, args, api) {
    var document = api.yaml.parse(content);
    if (!isObject(document) || !Array.isArray(document.rules)) {
        throw new Error("Mihomo document requires a rules array");
    }

    var additions = args.rules.map(function(rule) {
        return mihomoRule(rule, document.rules, args.routes);
    }).filter(function(candidate) {
        return document.rules.indexOf(candidate) < 0;
    });
    if (additions.length === 0) return undefined;

    document.rules.splice.apply(document.rules, [0, 0].concat(additions));
    return api.yaml.stringify(document);
}

function mihomoRule(rule, rules, routes) {
    var target = resolveRoute(rule.route, "mihomo", routes, function() {
        return mihomoFinalPolicy(rules);
    });
    var policy = target.reject ? "REJECT" : target.policy;

    if (rule.type === "domain") {
        return "DOMAIN," + cleanComponent(rule.value) + "," + policy;
    }
    if (rule.type === "domain_suffix") {
        return "DOMAIN-SUFFIX," + cleanComponent(rule.value) + "," + policy;
    }
    if (rule.type === "domain_keyword") {
        return "DOMAIN-KEYWORD," + cleanComponent(rule.value) + "," + policy;
    }
    if (rule.type === "ip_cidr") {
        return ipCIDRRuleName(rule.value) + "," + cleanComponent(rule.value) + "," + policy + ",no-resolve";
    }
    if (rule.type === "port") {
        return "DST-PORT," + normalizedPort(rule.value) + "," + policy;
    }
    throw new Error("unsupported Mihomo rule type: " + rule.type);
}

function mihomoFinalPolicy(rules) {
    for (var index = rules.length - 1; index >= 0; index -= 1) {
        var rule = rules[index];
        if (typeof rule === "string" && rule.indexOf("MATCH,") === 0) {
            return cleanPolicy(rule.slice("MATCH,".length));
        }
    }
    throw new Error("custom-routing.js cannot find Mihomo MATCH policy for route=proxy");
}

/* sing-box */

function applySingBox(content, args, api) {
    var document = api.json.parse(content);
    if (!isObject(document) || !isObject(document.route)) {
        throw new Error("sing-box document requires a route object");
    }
    if (document.route.rules === undefined) document.route.rules = [];
    if (!Array.isArray(document.route.rules)) throw new Error("sing-box route.rules must be an array");

    var additions = args.rules.map(function(rule) {
        return singBoxRule(rule, document, args.routes);
    }).filter(function(candidate) {
        return !document.route.rules.some(function(existing) {
            return exactEqual(existing, candidate);
        });
    });
    if (additions.length === 0) return undefined;

    var anchorIndex = singBoxInsertionIndex(document.route.rules);
    document.route.rules.splice.apply(document.route.rules, [anchorIndex, 0].concat(additions));
    return api.json.stringify(document);
}

function singBoxRule(rule, document, routes) {
    var target = resolveRoute(rule.route, "sing-box", routes, function() {
        return singBoxFinalPolicy(document);
    });
    var out = {};

    if (rule.type === "domain") {
        out.domain = [cleanComponent(rule.value)];
    } else if (rule.type === "domain_suffix") {
        out.domain_suffix = [cleanComponent(rule.value)];
    } else if (rule.type === "domain_keyword") {
        out.domain_keyword = [cleanComponent(rule.value)];
    } else if (rule.type === "ip_cidr") {
        out.ip_cidr = [cleanComponent(rule.value)];
    } else if (rule.type === "port") {
        out.port = normalizedPort(rule.value);
    } else {
        throw new Error("unsupported sing-box rule type: " + rule.type);
    }

    if (target.reject) {
        out.action = "reject";
    } else {
        out.action = "route";
        out.outbound = target.policy;
    }
    return out;
}

function singBoxFinalPolicy(document) {
    var route = document.route;
    if (typeof route.final === "string" && route.final.trim()) {
        return cleanPolicy(route.final);
    }

    for (var index = route.rules.length - 1; index >= 0; index -= 1) {
        var rule = route.rules[index];
        if (isSingBoxMatchAllRoute(rule)) return cleanPolicy(rule.outbound);
    }

    var outbounds = Array.isArray(document.outbounds) ? document.outbounds : [];
    for (var outboundIndex = 0; outboundIndex < outbounds.length; outboundIndex += 1) {
        var outbound = outbounds[outboundIndex];
        if (isObject(outbound) && typeof outbound.tag === "string" && outbound.tag.trim()) {
            return cleanPolicy(outbound.tag);
        }
    }
    throw new Error("custom-routing.js cannot find sing-box final policy for route=proxy");
}

function singBoxInsertionIndex(rules) {
    for (var index = 0; index < rules.length; index += 1) {
        var rule = rules[index];
        if (isObject(rule) && typeof rule.outbound === "string" &&
            !Object.prototype.hasOwnProperty.call(rule, "clash_mode")) {
            return index;
        }
    }
    return rules.length;
}

function isSingBoxMatchAllRoute(rule) {
    if (!isObject(rule) || typeof rule.outbound !== "string") return false;
    return Object.keys(rule).every(function(key) {
        return key === "outbound" || (key === "action" && rule.action === "route");
    });
}

/* Shadowrocket */

function applyShadowrocket(content, args, api) {
    var document = api.ini.parse(content);
    if (!isObject(document) || !Array.isArray(document.sections)) {
        throw new Error("invalid Shadowrocket INI document");
    }

    var sections = document.sections.filter(function(section) {
        return isObject(section) && typeof section.name === "string" &&
            section.name.toLowerCase() === "rule" && Array.isArray(section.lines);
    });
    if (sections.length === 0) {
        throw new Error("Shadowrocket document requires a [Rule] section");
    }

    var additions = args.rules.map(function(rule) {
        return shadowrocketRule(rule, sections, args.routes);
    }).filter(function(candidate) {
        return !sections.some(function(section) {
            return section.lines.indexOf(candidate) >= 0;
        });
    });
    if (additions.length === 0) return undefined;

    sections[0].lines.splice.apply(sections[0].lines, [0, 0].concat(additions));
    return api.ini.stringify(document);
}

function shadowrocketRule(rule, sections, routes) {
    var target = resolveRoute(rule.route, "shadowrocket", routes, function() {
        return shadowrocketFinalPolicy(sections);
    });
    var policy = target.reject ? "REJECT" : target.policy;

    if (rule.type === "domain") {
        return "DOMAIN," + cleanComponent(rule.value) + "," + policy;
    }
    if (rule.type === "domain_suffix") {
        return "DOMAIN-SUFFIX," + cleanComponent(rule.value) + "," + policy;
    }
    if (rule.type === "domain_keyword") {
        return "DOMAIN-KEYWORD," + cleanComponent(rule.value) + "," + policy;
    }
    if (rule.type === "ip_cidr") {
        return ipCIDRRuleName(rule.value) + "," + cleanComponent(rule.value) + "," + policy + ",no-resolve";
    }
    if (rule.type === "port") {
        return "DST-PORT," + normalizedPort(rule.value) + "," + policy;
    }
    throw new Error("unsupported Shadowrocket rule type: " + rule.type);
}

function shadowrocketFinalPolicy(sections) {
    for (var sectionIndex = sections.length - 1; sectionIndex >= 0; sectionIndex -= 1) {
        var lines = sections[sectionIndex].lines;
        for (var lineIndex = lines.length - 1; lineIndex >= 0; lineIndex -= 1) {
            var line = lines[lineIndex];
            if (typeof line !== "string") continue;
            var fields = line.split(",");
            if (fields[0].trim().toUpperCase() === "FINAL" && fields.length >= 2) {
                return cleanPolicy(fields.slice(1).join(","));
            }
        }
    }
    throw new Error("custom-routing.js cannot find Shadowrocket FINAL policy for route=proxy");
}

/* 参数、路由映射与校验 */

function resolveRoute(routeName, kind, routes, finalPolicy) {
    routeName = routeName.trim();
    if (routeName === "direct") {
        return {policy: kind === "sing-box" ? "direct" : "DIRECT", reject: false};
    }
    if (routeName === "reject") {
        return {policy: "", reject: true};
    }
    if (routeName === "proxy") {
        return {policy: cleanPolicy(finalPolicy()), reject: false};
    }

    var mapping = routes[routeName];
    var value = typeof mapping === "string" ? mapping :
        isObject(mapping) ? mapping[kind] : "";
    if (typeof value !== "string" || !value.trim()) {
        throw new Error("custom route " + routeName + " has no target for " + kind);
    }
    return {policy: cleanPolicy(value), reject: false};
}

function validateRules(rules) {
    if (rules.length === 0) throw new Error("rules must be a non-empty array");
    rules.forEach(function(rule, index) {
        if (!isObject(rule)) throw new Error("rules[" + index + "] must be an object");
        var supportedTypes = ["domain", "domain_suffix", "domain_keyword", "ip_cidr", "port"];
        if (supportedTypes.indexOf(rule.type) < 0) {
            throw new Error("rules[" + index + "] has unsupported type");
        }
        if (typeof rule.route !== "string" || !rule.route.trim()) {
            throw new Error("rules[" + index + "] requires route");
        }
        if (rule.type === "port") normalizedPort(rule.value);
        else cleanComponent(rule.value);
    });
}

function validateRoutes(routes) {
    Object.keys(routes).forEach(function(name) {
        if (!name.trim()) throw new Error("routes contains an empty route name");
        var value = routes[name];
        if (typeof value === "string") {
            cleanPolicy(value);
            return;
        }
        if (!isObject(value)) throw new Error("routes." + name + " must be a string or object");
        Object.keys(value).forEach(function(kind) {
            if (["mihomo", "sing-box", "shadowrocket"].indexOf(kind) < 0) {
                throw new Error("routes." + name + " has unsupported client: " + kind);
            }
            cleanPolicy(value[kind]);
        });
    });
}

function readJSONArray(value, name, api) {
    if (typeof value === "string") value = api.json.parse(value);
    if (!Array.isArray(value)) throw new Error(name + " must be a JSON array");
    return value;
}

function readJSONObject(value, name, api, fallback) {
    if (value === undefined || value === null || value === "") return fallback;
    if (typeof value === "string") value = api.json.parse(value);
    if (!isObject(value)) throw new Error(name + " must be a JSON object");
    return value;
}

function cleanComponent(value) {
    if (typeof value !== "string") throw new Error("rule value must be a string");
    var out = value.trim();
    if (!out || out.indexOf(",") >= 0 || out.indexOf("\n") >= 0 || out.indexOf("\r") >= 0) {
        throw new Error("invalid rule value: " + value);
    }
    return out;
}

function cleanPolicy(value) {
    if (typeof value !== "string") throw new Error("route policy must be a string");
    var out = value.trim();
    if (!out || out.indexOf(",") >= 0 || out.indexOf("\n") >= 0 || out.indexOf("\r") >= 0) {
        throw new Error("invalid route policy: " + value);
    }
    return out;
}

function normalizedPort(value) {
    var port = typeof value === "number" ? value : Number(String(value).trim());
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
        throw new Error("invalid destination port: " + value);
    }
    return port;
}

function ipCIDRRuleName(value) {
    return cleanComponent(value).indexOf(":") >= 0 ? "IP-CIDR6" : "IP-CIDR";
}

function exactEqual(left, right) {
    if (left === right) return true;
    if (Array.isArray(left) || Array.isArray(right)) {
        if (!Array.isArray(left) || !Array.isArray(right) || left.length !== right.length) return false;
        return left.every(function(value, index) { return exactEqual(value, right[index]); });
    }
    if (!isObject(left) || !isObject(right)) return false;
    var leftKeys = Object.keys(left);
    var rightKeys = Object.keys(right);
    if (leftKeys.length !== rightKeys.length) return false;
    return leftKeys.every(function(key) {
        return Object.prototype.hasOwnProperty.call(right, key) && exactEqual(left[key], right[key]);
    });
}

function requireFile(input) {
    if (!isObject(input) || input.stage !== "file" || !isObject(input.file) ||
        typeof input.file.content !== "string") {
        throw new Error(SCRIPT_NAME + " requires file-stage text input");
    }
}

function isObject(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
