/*
 * real-ip-domains.js
 *
 * 让指定域名绕过 FakeIP：
 * - Mihomo：合并 dns.fake-ip-filter
 * - sing-box：在当前 FakeIP DNS 规则前插入真实 DNS 规则
 * - Shadowrocket：合并 [General] always-real-ip
 *
 * params.args（Web 参数每行 key=value）：
 *   domain=host.example.com                  可选，精确域名
 *   domain_suffix=example.net,example.org    可选，根域名及所有子域名
 *   sing_box_server=dns-remote               可选，现有真实 DNS server tag
 *
 * domain / domain_suffix 可传逗号分隔文本、字符串数组或 JSON 数组字符串，
 * 两者合计至少一项。sing_box_server 省略时优先使用 dns.final；若未设置，
 * 仅在配置中恰好有一个可用真实 resolver 时自动选择。
 */

var SCRIPT_NAME = "real-ip-domains.js";

var ADAPTERS = {
    "mihomo": {
        parse: parseMihomo,
        apply: applyMihomo,
        stringify: stringifyMihomo
    },
    "sing-box": {
        parse: parseSingBox,
        apply: applySingBox,
        stringify: stringifySingBox
    },
    "shadowrocket": {
        parse: parseShadowrocket,
        apply: applyShadowrocket,
        stringify: stringifyShadowrocket
    }
};

function main(input, api) {
    requireFile(input);
    var adapter = ADAPTERS[input.file.kind];
    if (!adapter) throw new Error(SCRIPT_NAME + " does not support file kind: " + input.file.kind);

    var args = readArgs(input.args || {}, api);
    var content = applyOperation(input.file.content, args, api, adapter);
    if (content !== undefined) input.file.content = content;
    return input;
}

function applyOperation(content, args, api, adapter) {
    var state = adapter.parse(content, api);
    if (adapter.apply(state, args, api) === false) return undefined;
    return adapter.stringify(state, api);
}

function readArgs(args, api) {
    var domains = {
        exact: readDomains(args.domain, "domain", api),
        suffix: readDomains(args.domain_suffix, "domain_suffix", api)
    };
    if (domains.exact.length + domains.suffix.length === 0) {
        throw new Error("domain 和 domain_suffix 至少需要填写一项");
    }
    return {
        domains: domains,
        singBoxServer: args.sing_box_server
    };
}

/* Mihomo adapter */

function parseMihomo(content, api) {
    return api.yaml.parse(content);
}

function applyMihomo(document, args) {
    if (!isObject(document) || !isObject(document.dns)) {
        throw new Error("Mihomo document requires a dns object");
    }
    if (document.dns["enhanced-mode"] !== "fake-ip") return false;
    if (document.dns["fake-ip-filter-mode"] === "rule") {
        throw new Error(SCRIPT_NAME + " does not support Mihomo fake-ip-filter-mode=rule");
    }

    var filter = document.dns["fake-ip-filter"];
    if (filter === undefined) filter = [];
    if (!Array.isArray(filter) || filter.some(function(item) { return typeof item !== "string"; })) {
        throw new Error("Mihomo dns.fake-ip-filter must be an array of strings");
    }

    args.domains.exact.concat(args.domains.suffix.map(function(domain) {
        return "+." + domain;
    })).forEach(function(item) {
        appendUnique(filter, item);
    });
    document.dns["fake-ip-filter"] = filter;
}

function stringifyMihomo(document, api) {
    return api.yaml.stringify(document);
}

/* sing-box adapter */

function parseSingBox(content, api) {
    return api.json.parse(content);
}

function applySingBox(document, args) {
    if (!isObject(document) || !isObject(document.dns) || !Array.isArray(document.dns.servers)) {
        throw new Error("sing-box document requires dns.servers");
    }
    var rules = document.dns.rules === undefined ? [] : document.dns.rules;
    if (!Array.isArray(rules) || rules.some(function(rule) { return !isObject(rule); })) {
        throw new Error("sing-box dns.rules must be an array of objects");
    }

    var fakeTags = document.dns.servers.filter(function(server) {
        return isObject(server) && server.type === "fakeip" &&
            typeof server.tag === "string" && server.tag.trim();
    }).map(function(server) {
        return server.tag;
    });
    var firstFakeRule = rules.findIndex(function(rule) {
        return typeof rule.server === "string" && fakeTags.indexOf(rule.server) >= 0;
    });
    if (firstFakeRule < 0) return false;

    var realServer = selectRealDNSServer(document.dns, readOptionalTag(args.singBoxServer));
    var addition = {action: "route", server: realServer};
    if (args.domains.exact.length > 0) addition.domain = args.domains.exact.slice();
    if (args.domains.suffix.length > 0) addition.domain_suffix = args.domains.suffix.slice();

    var retained = rules.filter(function(rule) { return !exactEqual(rule, addition); });
    firstFakeRule = retained.findIndex(function(rule) {
        return typeof rule.server === "string" && fakeTags.indexOf(rule.server) >= 0;
    });
    retained.splice(firstFakeRule, 0, addition);
    document.dns.rules = retained;
}

function stringifySingBox(document, api) {
    return api.json.stringify(document);
}

function selectRealDNSServer(dns, explicitTag) {
    var servers = dns.servers.filter(function(server) {
        return isObject(server) && isRealDNSServer(server);
    });
    var selectedTag = explicitTag ||
        (typeof dns.final === "string" && dns.final.trim() ? dns.final.trim() : "");
    var matches;
    if (selectedTag) {
        matches = servers.filter(function(server) { return server.tag === selectedTag; });
    } else {
        matches = servers;
    }
    if (matches.length !== 1 || typeof matches[0].tag !== "string" || !matches[0].tag.trim()) {
        throw new Error("sing-box 需要唯一的真实 DNS server；请设置 sing_box_server");
    }
    return matches[0].tag;
}

function isRealDNSServer(server) {
    var types = ["local", "udp", "tcp", "tls", "https", "quic", "h3", "dhcp"];
    if (server.type === "resolved") return server.accept_default_resolvers === true &&
        typeof server.tag === "string" && server.tag.trim() !== "";
    return types.indexOf(server.type) >= 0 && typeof server.tag === "string" && server.tag.trim() !== "";
}

/* Shadowrocket adapter */

function parseShadowrocket(content, api) {
    return {
        content: content,
        document: api.ini.parse(content),
        patch: ""
    };
}

function applyShadowrocket(state, args) {
    if (!isObject(state.document) || !Array.isArray(state.document.sections)) {
        throw new Error("invalid Shadowrocket INI document");
    }

    var values = existingINIValues(state.document, "General", "always-real-ip");
    args.domains.exact.forEach(function(domain) { appendUnique(values, domain); });
    args.domains.suffix.forEach(function(domain) {
        appendUnique(values, domain);
        appendUnique(values, "*." + domain);
    });
    state.patch = "[General]\nalways-real-ip = " + values.join(",") + "\n";
}

function stringifyShadowrocket(state, api) {
    return api.ini.override(state.content, state.patch);
}

function existingINIValues(document, sectionName, key) {
    var out = [];
    document.sections.forEach(function(section) {
        if (!isObject(section) || typeof section.name !== "string" ||
            section.name.toLowerCase() !== sectionName.toLowerCase() || !Array.isArray(section.lines)) {
            return;
        }
        section.lines.forEach(function(line) {
            if (typeof line !== "string") return;
            var separator = line.indexOf("=");
            if (separator < 0 || line.slice(0, separator).trim().toLowerCase() !== key.toLowerCase()) return;
            line.slice(separator + 1).split(",").forEach(function(value) {
                var item = value.trim();
                if (item) appendUnique(out, item);
            });
        });
    });
    return out;
}

/* 参数与通用工具 */

function readDomains(value, name, api) {
    if (value === undefined || value === null || value === "") return [];
    if (typeof value === "string") {
        var text = value.trim();
        value = text.charAt(0) === "[" ? api.json.parse(text) : text.split(",");
    }
    if (!Array.isArray(value)) value = [value];
    return value.map(function(item) {
        if (typeof item !== "string") throw new Error(name + " 必须是域名字符串");
        return cleanDomain(item);
    }).filter(function(item, index, values) {
        return values.indexOf(item) === index;
    });
}

function cleanDomain(value) {
    var out = value.trim().toLowerCase().replace(/^\+\.|^\*\.|^\./, "").replace(/\.$/, "");
    if (!out || out.length > 253 || /^[0-9.]+$/.test(out) ||
        !out.split(".").every(function(label) {
            return /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label);
        })) {
        throw new Error("invalid domain: " + value);
    }
    return out;
}

function readOptionalTag(value) {
    if (value === undefined || value === null || value === "") return "";
    if (typeof value !== "string" || !value.trim() || /[\r\n]/.test(value)) {
        throw new Error("sing_box_server 必须是非空字符串");
    }
    return value.trim();
}

function appendUnique(values, value) {
    if (values.indexOf(value) < 0) values.push(value);
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
