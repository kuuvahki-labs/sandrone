/*
 * custom-hosts.js
 *
 * 把 params.args.hosts 中的“精确主机名 -> 单个 IP”映射写入 Mihomo、
 * sing-box 或 Shadowrocket。这是三端稳定的公共子集。
 *
 * params.args（Web 参数每行 key=value）：
 *   hosts={"router.lan":"192.0.2.1","service.lan":"2001:db8::1"}
 *   sing_box_tag=custom-hosts       可选，本脚本使用的 DNS server tag
 *
 * hosts 可直接传对象，也可传 JSON 对象字符串。脚本拥有 sing_box_tag
 * 对应的 hosts DNS server 与引用规则；重复执行会更新而不会累积旧映射。
 */

var SCRIPT_NAME = "custom-hosts.js";

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
    return {
        hosts: readHosts(args.hosts, api),
        singBoxTag: readTag(args.sing_box_tag, "custom-hosts")
    };
}

/* Mihomo adapter */

function parseMihomo(content, api) {
    return api.yaml.parse(content);
}

function applyMihomo(document, args) {
    if (!isObject(document)) throw new Error("invalid Mihomo document");
    if (document.hosts === undefined) document.hosts = {};
    if (!isObject(document.hosts)) throw new Error("Mihomo hosts must be an object");

    Object.keys(args.hosts).forEach(function(domain) {
        document.hosts[domain] = args.hosts[domain];
    });
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
    if (!Array.isArray(rules)) throw new Error("sing-box dns.rules must be an array");

    var matchingServers = document.dns.servers.filter(function(server) {
        return isObject(server) && server.tag === args.singBoxTag;
    });
    if (matchingServers.length > 1) {
        throw new Error("duplicate sing-box DNS server tag: " + args.singBoxTag);
    }
    if (matchingServers.length === 1 && matchingServers[0].type !== "hosts") {
        throw new Error("sing-box DNS server tag collision: " + args.singBoxTag);
    }

    var server = {type: "hosts", tag: args.singBoxTag, predefined: args.hosts};
    document.dns.servers = document.dns.servers.filter(function(candidate) {
        return !isObject(candidate) || candidate.tag !== args.singBoxTag;
    }).concat([server]);

    var retainedRules = rules.filter(function(rule) {
        if (!isObject(rule) || rule.server !== args.singBoxTag) return true;
        if (!isOwnedSingBoxRule(rule)) {
            throw new Error("sing-box DNS rule tag collision: " + args.singBoxTag);
        }
        return false;
    });
    document.dns.rules = [{
        domain: Object.keys(args.hosts),
        action: "route",
        server: args.singBoxTag
    }].concat(retainedRules);
}

function stringifySingBox(document, api) {
    return api.json.stringify(document);
}

function isOwnedSingBoxRule(rule) {
    var allowed = {domain: true, action: true, server: true};
    return rule.action === "route" && Array.isArray(rule.domain) &&
        Object.keys(rule).every(function(key) { return allowed[key] === true; });
}

/* Shadowrocket adapter */

function parseShadowrocket(content) {
    return {content: content, patch: ""};
}

function applyShadowrocket(state, args) {
    var lines = ["[Host]"];
    Object.keys(args.hosts).forEach(function(domain) {
        lines.push(domain + " = " + args.hosts[domain]);
    });
    state.patch = lines.join("\n") + "\n";
}

function stringifyShadowrocket(state, api) {
    return api.ini.override(state.content, state.patch);
}

function readHosts(value, api) {
    if (typeof value === "string") value = api.json.parse(value);
    if (!isObject(value) || Object.keys(value).length === 0) {
        throw new Error("hosts must be a non-empty JSON object");
    }

    var hosts = {};
    Object.keys(value).forEach(function(rawDomain) {
        var domain = cleanHostName(rawDomain);
        var address = cleanIPAddress(value[rawDomain]);
        if (Object.prototype.hasOwnProperty.call(hosts, domain) && hosts[domain] !== address) {
            throw new Error("duplicate host mapping: " + domain);
        }
        hosts[domain] = address;
    });
    return hosts;
}

function cleanHostName(value) {
    var out = typeof value === "string" ? value.trim().toLowerCase().replace(/\.$/, "") : "";
    if (!out || /[\s=,*+\/]/.test(out) || out.indexOf("..") >= 0 ||
        !out.split(".").every(function(label) {
            return /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label);
        })) {
        throw new Error("invalid exact host name: " + value);
    }
    return out;
}

function cleanIPAddress(value) {
    if (typeof value !== "string") throw new Error("host address must be an IP string");
    var out = value.trim().toLowerCase();
    if (!validIPv4(out) && !validIPv6(out)) throw new Error("invalid host IP address: " + value);
    return out;
}

function validIPv4(value) {
    var parts = value.split(".");
    return parts.length === 4 && parts.every(function(part) {
        return /^(0|[1-9][0-9]{0,2})$/.test(part) && Number(part) <= 255;
    });
}

function validIPv6(value) {
    if (value.indexOf(".") !== -1) {
        var lastColon = value.lastIndexOf(":");
        if (lastColon < 0 || !validIPv4(value.slice(lastColon + 1))) return false;
        value = value.slice(0, lastColon + 1) + "0:0";
    }
    var halves = value.split("::");
    if (halves.length > 2) return false;
    var groups = [];
    halves.forEach(function(half) {
        if (half) groups = groups.concat(half.split(":"));
    });
    return (halves.length === 2 ? groups.length < 8 : groups.length === 8) &&
        groups.every(function(group) { return /^[0-9a-f]{1,4}$/.test(group); });
}

function readTag(value, fallback) {
    if (value === undefined || value === null || value === "") return fallback;
    if (typeof value !== "string" || !value.trim() || /[\r\n]/.test(value)) {
        throw new Error("sing_box_tag must be a non-empty string");
    }
    return value.trim();
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
