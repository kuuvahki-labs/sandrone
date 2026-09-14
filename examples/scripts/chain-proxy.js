/*
 * Sandrone Mihomo / sing-box 链式代理文件脚本
 *
 * 作为 file-stage script processor 使用，直接修改最终配置：
 *   1. 按 landing_pattern 识别落地代理并设置前置代理；
 *   2. 生成“前置代理”和“落地节点”两个手动选择组。
 *
 * ProcessorSpec.params.args：
 *   landing_pattern   必填，落地代理名称正则（忽略大小写）
 *   front_proxies     可选，前置节点/分组数组；省略时使用全部非落地代理
 *   front_group       可选，默认“前置代理”
 *   landing_group     可选，默认“落地节点”
 *
 * front_proxies 也可传 JSON 数组字符串，例如：
 *   ["香港节点", "日本节点"]
 *
 * Mihomo 为落地节点写入 dialer-proxy 并创建 select 组；sing-box 为代理
 * outbound 写入 detour 并创建 selector outbound。其他客户端不受支持。
 */

var SCRIPT_NAME = "chain-proxy.js";
var GROUP_TYPES = {selector: true, urltest: true};
var NON_PROXY_TYPES = {selector: true, urltest: true, direct: true, block: true, dns: true};

var ADAPTERS = {
    "mihomo": {
        parse: parseMihomo,
        stringify: stringifyMihomo,
        candidates: mihomoCandidates,
        nameOf: mihomoNameOf,
        setDetour: setMihomoDetour,
        removeLandingMembers: removeMihomoLandingMembers,
        upsertGroups: upsertMihomoGroups,
        noLandingMessage: "landing_pattern 未匹配任何 Mihomo 节点，文件保持不变",
        emptyFrontMessage: "前置代理组不能为空"
    },
    "sing-box": {
        parse: parseSingBox,
        stringify: stringifySingBox,
        candidates: singBoxCandidates,
        nameOf: singBoxNameOf,
        setDetour: setSingBoxDetour,
        removeLandingMembers: removeSingBoxLandingMembers,
        upsertGroups: upsertSingBoxGroups,
        noLandingMessage: "landing_pattern 未匹配任何 sing-box 代理 outbound，文件保持不变",
        emptyFrontMessage: "前置代理 selector 不能为空"
    }
};

function main(input, api) {
    requireFile(input);
    var adapter = ADAPTERS[input.file.kind];
    if (!adapter) {
        throw new Error(SCRIPT_NAME + " does not support file kind: " + input.file.kind);
    }

    var args = readArgs(input.args || {});
    var content = applyOperation(input.file.content, args, api, adapter);
    if (content !== undefined) input.file.content = content;
    return input;
}

function readArgs(args) {
    var frontGroup = readName(args.front_group, "前置代理");
    var landingGroup = readName(args.landing_group, "落地节点");
    if (frontGroup === landingGroup) throw new Error("前置组和落地组不能同名");
    return {
        matcher: readPattern(args.landing_pattern),
        frontProxies: args.front_proxies,
        frontGroup: frontGroup,
        landingGroup: landingGroup
    };
}

function applyOperation(content, args, api, adapter) {
    var document = adapter.parse(content, api);
    var landing = [];
    var ordinary = [];

    adapter.candidates(document).forEach(function(candidate) {
        var name = adapter.nameOf(candidate);
        if (args.matcher.test(name)) {
            adapter.setDetour(candidate, args.frontGroup);
            landing.push(name);
        } else {
            ordinary.push(name);
        }
    });
    if (landing.length === 0) {
        api.warn({code: "chain_proxy_no_landing_nodes", message: adapter.noLandingMessage});
        return undefined;
    }

    var front = readStringList(args.frontProxies, ordinary);
    if (front.length === 0) throw new Error(adapter.emptyFrontMessage);
    var landingSet = toSet(landing);
    front.forEach(function(name) {
        if (name === args.frontGroup || name === args.landingGroup || landingSet[name]) {
            throw new Error("前置代理不能引用链式分组或落地代理: " + name);
        }
        adapter.removeLandingMembers(document, name, landing, landingSet);
    });

    adapter.upsertGroups(document, args.frontGroup, args.landingGroup, front, landing);
    return adapter.stringify(document, api);
}

/* Mihomo adapter */

function parseMihomo(content, api) {
    var document = api.yaml.parse(content);
    return document || {};
}

function stringifyMihomo(document, api) {
    return api.yaml.stringify(document);
}

function mihomoCandidates(document) {
    var proxies = Array.isArray(document.proxies) ? document.proxies : [];
    return proxies.filter(function(proxy) {
        return proxy && typeof proxy.name === "string";
    });
}

function mihomoNameOf(proxy) {
    return proxy.name;
}

function setMihomoDetour(proxy, frontGroup) {
    proxy["dialer-proxy"] = frontGroup;
}

function removeMihomoLandingMembers(document, name, landing, landingSet) {
    var groups = Array.isArray(document["proxy-groups"]) ? document["proxy-groups"] : [];
    var group = findBy(groups, "name", name);
    if (!group) return;
    if (Array.isArray(group.proxies)) {
        group.proxies = group.proxies.filter(function(member) { return !landingSet[member]; });
    }
    if (group["include-all"] === true || group["include-all-proxies"] === true) {
        appendLandingExclusion(group, landing);
    }
}

function upsertMihomoGroups(document, frontGroup, landingGroup, front, landing) {
    var groups = Array.isArray(document["proxy-groups"]) ? document["proxy-groups"] : [];
    groups = upsert(groups, "name", frontGroup, {name: frontGroup, type: "select", proxies: front});
    groups = upsert(groups, "name", landingGroup, {name: landingGroup, type: "select", proxies: landing});
    document["proxy-groups"] = groups;
}

function appendLandingExclusion(group, landing) {
    var pattern = "^(?:" + landing.map(escapeRegExp).join("|") + ")$";
    var current = typeof group["exclude-filter"] === "string" ? group["exclude-filter"] : "";
    if (current.indexOf(pattern) === -1) group["exclude-filter"] = current ? current + "|" + pattern : pattern;
}

/* sing-box adapter */

function parseSingBox(content, api) {
    var document = api.json.parse(content);
    return document || {};
}

function stringifySingBox(document, api) {
    return api.json.stringify(document);
}

function singBoxCandidates(document) {
    var outbounds = Array.isArray(document.outbounds) ? document.outbounds : [];
    return outbounds.filter(isProxyOutbound);
}

function singBoxNameOf(outbound) {
    return outbound.tag;
}

function setSingBoxDetour(outbound, frontGroup) {
    outbound.detour = frontGroup;
}

function removeSingBoxLandingMembers(document, tag, landing, landingSet) {
    var outbounds = Array.isArray(document.outbounds) ? document.outbounds : [];
    var group = findBy(outbounds, "tag", tag);
    if (!group || !GROUP_TYPES[group.type] || !Array.isArray(group.outbounds)) return;
    group.outbounds = group.outbounds.filter(function(member) { return !landingSet[member]; });
}

function upsertSingBoxGroups(document, frontGroup, landingGroup, front, landing) {
    var outbounds = Array.isArray(document.outbounds) ? document.outbounds : [];
    outbounds = upsert(outbounds, "tag", frontGroup, {
        type: "selector", tag: frontGroup, outbounds: front
    });
    outbounds = upsert(outbounds, "tag", landingGroup, {
        type: "selector", tag: landingGroup, outbounds: landing
    });
    document.outbounds = outbounds;
}

function isProxyOutbound(outbound) {
    return outbound
        && typeof outbound.type === "string"
        && !NON_PROXY_TYPES[outbound.type]
        && typeof outbound.tag === "string"
        && outbound.tag.trim() !== "";
}

/* 参数与通用工具 */

function upsert(values, key, name, replacement) {
    var found = false;
    var result = values.map(function(value) {
        if (!value || value[key] !== name) return value;
        if (found) return null;
        found = true;
        return replacement;
    }).filter(function(value) { return value !== null; });
    if (!found) result.push(replacement);
    return result;
}

function findBy(values, key, name) {
    for (var index = 0; index < values.length; index += 1) {
        if (values[index] && values[index][key] === name) return values[index];
    }
    return null;
}

function readStringList(value, fallback) {
    if (value === undefined || value === null || value === "") return fallback.slice();
    if (typeof value === "string" && value.trim().charAt(0) === "[") value = JSON.parse(value);
    if (!Array.isArray(value)) value = [value];
    return value.map(function(item) {
        if (typeof item !== "string" || !item.trim()) throw new Error("front_proxies 必须是非空字符串数组");
        return item.trim();
    }).filter(function(item, index, values) { return values.indexOf(item) === index; });
}

function readPattern(value) {
    if (typeof value !== "string" || !value.trim()) throw new Error("landing_pattern 必须是非空正则表达式");
    return new RegExp(value, "i");
}

function readName(value, fallback) {
    value = value === undefined || value === null ? fallback : String(value).trim();
    if (!value) throw new Error("分组名不能为空");
    return value;
}

function toSet(values) {
    var result = Object.create(null);
    values.forEach(function(value) { result[value] = true; });
    return result;
}

function escapeRegExp(value) {
    return String(value).replace(/[|\\{}()[\]^$+*?.-]/g, "\\$&");
}

function requireFile(input) {
    if (!input || input.stage !== "file" || !input.file ||
        typeof input.file.content !== "string") {
        throw new Error(SCRIPT_NAME + " requires file-stage text input");
    }
}
