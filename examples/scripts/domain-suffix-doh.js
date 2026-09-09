/*
 * Sandrone 按域名后缀指定直连 DoH（file-stage）。支持 Mihomo / sing-box 1.12+ / Shadowrocket。
 * 放在文件处理链末尾；通过 input.file.kind 自动选择客户端。
 *
 * params.args（Web 参数每行 key=value）：
 *   suffixes=example.com,example.net
 *   doh=https://dns.example.org/dns-query
 *   bootstrap=dns-cn                    可选，仅 sing-box，现有 DNS server tag
 *   tag=sandrone-suffix-doh              可选，仅 sing-box，本脚本专用 DNS tag
 *
 * suffixes 必填，支持逗号分隔或字符串数组/JSON 数组字符串，包含域名本身及子域名。
 * doh 必填，HTTPS 地址，可含端口和路径；不接受认证信息、查询参数或 # 附加参数。
 * Mihomo 自动附加 #DIRECT，并开启 direct-nameserver-follow-policy；保留其他策略，
 * 更具体的现有域名策略仍按客户端优先级生效。节点域名专用 resolver 不在修改范围。
 * sing-box 新增直连 HTTPS DNS server，并把后缀规则放在 DNS rules 最前面；
 * DoH 使用域名时，bootstrap 必须指向已存在的独立解析器，默认 dns-cn。
 * tag 及引用它的 DNS 规则归本脚本管理；多个脚本实例必须使用不同 tag。
 * Mihomo / sing-box 不修改网站流量路由，仅对交给客户端本地 DNS 的解析生效。
 * Shadowrocket 写入 [Host] 的域名和 *.域名两条 server:https:// 映射，
 * 并在 [Rule] 最前面写入 DOMAIN-SUFFIX,域名,DIRECT，因此目标网站也会直连。
 * 此脚本处理完整文件，直接合并最终 [Host]；Sandrone INI 追加补丁使用 [Host+]。
 * Shadowrocket URL 中的逗号需写为 %2C，以免被解释为多个 DNS 服务器。
 * Shadowrocket 保留全局 DNS、其他 Host 和规则；重复执行会更新目标项，不重复追加。
 */

function main(input, api) {
    if (!input || input.stage !== "file" || !input.file || typeof input.file.content !== "string") {
        throw new Error("domain-suffix-doh requires file-stage text input");
    }
    var kind = input.file.kind;
    if (kind !== "mihomo" && kind !== "sing-box" && kind !== "shadowrocket") {
        throw new Error("domain-suffix-doh 仅支持 mihomo、sing-box 和 shadowrocket");
    }
    var args = input.args || {};
    var suffixes = readSuffixes(args.suffixes);
    var endpoint = readEndpoint(args.doh);
    if (kind === "shadowrocket") {
        input.file.content = configureShadowrocket(input.file.content, api, suffixes, endpoint);
        return input;
    }
    var codec = kind === "mihomo" ? api.yaml : api.json;
    var doc = requireObject(codec.parse(input.file.content), "文件正文");
    var dns = requireObject(doc.dns, "dns");

    if (kind === "mihomo") {
        if (dns.enable !== true) throw new Error("请先启用 Mihomo dns.enable");
        var policy = dns["nameserver-policy"] === undefined ? {} :
            requireObject(dns["nameserver-policy"], "dns.nameserver-policy");
        suffixes.forEach(function(suffix) {
            policy["+." + suffix] = [endpoint.url + "#DIRECT"];
        });
        dns["nameserver-policy"] = policy;
        dns["direct-nameserver-follow-policy"] = true;
    } else {
        configureSingBox(dns, args, suffixes, endpoint);
    }
    input.file.content = codec.stringify(doc);
    return input;
}

function configureShadowrocket(content, api, suffixes, endpoint) {
    if (endpoint.url.indexOf(",") !== -1) {
        throw new Error("Shadowrocket doh URL 不能包含未编码的逗号，请使用 %2C");
    }
    var doc = api.ini.parse(content);
    var hostKeys = Object.create(null);
    var hostLines = [];
    var ruleLines = [];
    suffixes.forEach(function(suffix) {
        [suffix, "*." + suffix].forEach(function(key) {
            hostKeys[key] = true;
            hostLines.push(key + " = server:" + endpoint.url);
        });
        ruleLines.push("DOMAIN-SUFFIX," + suffix + ",DIRECT");
    });
    var hostSection;
    var ruleSection;
    doc.sections.forEach(function(section) {
        var name = section.name.toLowerCase();
        if (name === "host") {
            if (!hostSection) hostSection = section;
            section.lines = section.lines.filter(function(line) {
                var match = /^\s*([^#;=][^=]*?)\s*=/.exec(line);
                return !match || !hostKeys[match[1].trim().toLowerCase()];
            });
        } else if (name === "rule") {
            if (!ruleSection) ruleSection = section;
            section.lines = section.lines.filter(function(line) {
                var fields = line.split(",");
                return fields.length < 3 || fields[0].trim().toUpperCase() !== "DOMAIN-SUFFIX" ||
                    suffixes.indexOf(fields[1].trim().toLowerCase()) === -1;
            });
        }
    });
    if (!hostSection) {
        hostSection = {name: "Host", lines: []};
        doc.sections.push(hostSection);
    }
    if (!ruleSection) {
        ruleSection = {name: "Rule", lines: []};
        doc.sections.push(ruleSection);
    }
    hostSection.lines = hostSection.lines.concat(hostLines);
    ruleSection.lines = ruleLines.concat(ruleSection.lines);
    return api.ini.stringify(doc);
}

function configureSingBox(dns, args, suffixes, endpoint) {
    var tag = readTag(args.tag, "sandrone-suffix-doh");
    var servers = requireArray(dns.servers, "dns.servers");
    var rules = dns.rules === undefined ? [] : requireArray(dns.rules, "dns.rules");
    servers.forEach(function(server) {
        requireObject(server, "dns.servers item");
        if (!server.type) throw new Error("需要 sing-box 1.12+ typed DNS server 配置");
        if (server.tag === tag && server.type !== "https") {
            throw new Error("tag 已被非 HTTPS DNS server 使用，请更换 tag");
        }
    });
    var server = {
        type: "https", tag: tag, server: endpoint.host,
        server_port: endpoint.port, path: endpoint.path
    };
    // 新版 HTTPS DNS 默认使用自己的直连 dialer；不继承 route.final。
    // 不设置 detour，避免依赖某个同名但实际为代理的 outbound。
    if (!endpoint.ip) {
        var bootstrap = readTag(args.bootstrap, "dns-cn");
        if (bootstrap === tag || !servers.some(function(item) { return item.tag === bootstrap; })) {
            throw new Error("bootstrap 必须指向已存在且不同于 tag 的 DNS server");
        }
        server.domain_resolver = bootstrap;
    }
    dns.servers = servers.filter(function(item) { return item.tag !== tag; }).concat([server]);
    dns.rules = [{domain_suffix: suffixes, action: "route", server: tag}].concat(
        rules.filter(function(rule) {
            requireObject(rule, "dns.rules item");
            return rule.server !== tag;
        })
    );
}

function readSuffixes(value) {
    if (typeof value === "string") {
        value = value.trim();
        value = value.charAt(0) === "[" ? JSON.parse(value) : value.split(",");
    }
    if (!Array.isArray(value) || value.length === 0) throw new Error("suffixes 必须是非空域名后缀列表");
    return value.map(function(item) {
        if (typeof item !== "string") throw new Error("suffixes 必须是域名字符串");
        var suffix = item.trim().toLowerCase().replace(/^\+\.|^\*\.|^\./, "").replace(/\.$/, "");
        if (!validDomain(suffix) || /^[0-9.]+$/.test(suffix)) {
            throw new Error("suffixes 仅接受域名后缀，国际化域名请使用 Punycode");
        }
        return suffix;
    }).filter(function(item, index, values) { return values.indexOf(item) === index; });
}

function readEndpoint(value) {
    // 沙箱没有 Node.js URL API；只接纳各端可直接表达的 HTTPS URL 子集。
    if (typeof value !== "string") throw new Error("doh 必须是 HTTPS URL");
    var url = value.trim();
    var match = /^https:\/\/(\[[0-9a-fA-F:.]+\]|[a-zA-Z0-9.-]+)(?::([0-9]+))?(\/[^?#\s\\]*)?$/.exec(url);
    if (!match) throw new Error("doh 必须是无认证信息、查询参数和片段的 HTTPS URL");
    var host = match[1].toLowerCase();
    var ipv6 = host.charAt(0) === "[";
    var ipv4 = /^[0-9.]+$/.test(host);
    if (ipv4 && !validIPv4(host) || ipv6 && !validIPv6(host.slice(1, -1)) || !ipv4 && !ipv6 && !validDomain(host)) {
        throw new Error("doh 主机名无效");
    }
    var port = match[2] === undefined ? 443 : Number(match[2]);
    if (port < 1 || port > 65535) throw new Error("doh 端口必须为 1–65535");
    var path = match[3] || "/dns-query";
    return {
        url: "https://" + host + (match[2] ? ":" + port : "") + path,
        host: ipv6 ? host.slice(1, -1) : host, port: port, path: path, ip: ipv4 || ipv6
    };
}

function validDomain(value) {
    return value.length <= 253 && value.split(".").every(function(label) {
        return /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label);
    });
}

function validIPv4(value) {
    var parts = value.split(".");
    return parts.length === 4 && parts.every(function(part) {
        return /^(0|[1-9][0-9]{0,2})$/.test(part) && Number(part) <= 255;
    });
}

function validIPv6(value) {
    // 支持压缩地址和 IPv4 尾部；不接受接口 zone ID。
    if (value.indexOf(".") !== -1) {
        var lastColon = value.lastIndexOf(":");
        if (!validIPv4(value.slice(lastColon + 1))) return false;
        value = value.slice(0, lastColon + 1) + "0:0";
    }
    var halves = value.split("::");
    if (halves.length > 2) return false;
    var groups = [];
    halves.forEach(function(half) { if (half) groups = groups.concat(half.split(":")); });
    return (halves.length === 2 ? groups.length < 8 : groups.length === 8) &&
        groups.every(function(group) { return /^[0-9a-f]{1,4}$/.test(group); });
}

function readTag(value, fallback) {
    if (value === undefined) return fallback;
    if (typeof value !== "string" || !value.trim()) throw new Error("tag/bootstrap 必须是非空字符串");
    return value.trim();
}

function requireObject(value, name) {
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error(name + " 必须是对象");
    return value;
}

function requireArray(value, name) {
    if (!Array.isArray(value)) throw new Error(name + " 必须是数组");
    return value;
}
