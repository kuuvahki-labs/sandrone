/*
 * Sandrone sing-box 链式代理文件脚本
 *
 * 作为 file-stage script processor 使用，直接修改最终 sing-box JSON：
 *   1. 按 landing_pattern 识别落地 outbound 并写入 detour；
 *   2. 生成“前置代理”和“落地节点”两个 selector outbound。
 *
 * ProcessorSpec.params.args：
 *   landing_pattern   必填，落地 outbound tag 正则（忽略大小写）
 *   front_proxies     可选，前置 outbound/selector tag 数组；省略时使用
 *                     全部非落地代理 outbound
 *   front_group       可选，默认“前置代理”
 *   landing_group     可选，默认“落地节点”
 *
 * front_proxies 也可传 JSON 数组字符串，例如：
 *   ["香港节点", "日本节点"]
 *
 * 如需在路由或其他 selector 中选择链式出口，引用 landing_group 即可。
 */

var GROUP_TYPES = {selector: true, urltest: true};
var NON_PROXY_TYPES = {selector: true, urltest: true, direct: true, block: true, dns: true};

function main(input, api) {
    requireFile(input, "sing-box");
    var args = input.args || {};
    var matcher = readPattern(args.landing_pattern);
    var frontGroup = readName(args.front_group, "前置代理");
    var landingGroup = readName(args.landing_group, "落地节点");
    if (frontGroup === landingGroup) throw new Error("前置组和落地组不能同名");

    var doc = api.json.parse(input.file.content);
    var outbounds = doc && Array.isArray(doc.outbounds) ? doc.outbounds : [];
    var landing = [];
    var ordinary = [];

    outbounds.forEach(function(outbound) {
        if (!isProxyOutbound(outbound)) return;
        if (matcher.test(outbound.tag)) {
            outbound.detour = frontGroup;
            landing.push(outbound.tag);
        } else {
            ordinary.push(outbound.tag);
        }
    });
    if (landing.length === 0) {
        api.warn({code: "chain_proxy_no_landing_nodes", message: "landing_pattern 未匹配任何 sing-box 代理 outbound，文件保持不变"});
        return input;
    }

    var front = readStringList(args.front_proxies, ordinary);
    if (front.length === 0) throw new Error("前置代理 selector 不能为空");
    var landingSet = toSet(landing);
    front.forEach(function(tag) {
        if (tag === frontGroup || tag === landingGroup || landingSet[tag]) {
            throw new Error("前置代理不能引用链式 selector 或落地 outbound: " + tag);
        }
        var group = findBy(outbounds, "tag", tag);
        if (!group || !GROUP_TYPES[group.type] || !Array.isArray(group.outbounds)) return;
        group.outbounds = group.outbounds.filter(function(member) { return !landingSet[member]; });
    });

    outbounds = upsert(outbounds, "tag", frontGroup, {
        type: "selector", tag: frontGroup, outbounds: front
    });
    outbounds = upsert(outbounds, "tag", landingGroup, {
        type: "selector", tag: landingGroup, outbounds: landing
    });
    doc.outbounds = outbounds;
    input.file.content = api.json.stringify(doc);
    return input;
}

function isProxyOutbound(outbound) {
    return outbound
        && typeof outbound.type === "string"
        && !NON_PROXY_TYPES[outbound.type]
        && typeof outbound.tag === "string"
        && outbound.tag.trim() !== "";
}

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
    if (!value) throw new Error("selector tag 不能为空");
    return value;
}

function toSet(values) {
    var result = Object.create(null);
    values.forEach(function(value) { result[value] = true; });
    return result;
}

function requireFile(input, kind) {
    if (!input || input.stage !== "file" || !input.file || typeof input.file.content !== "string") {
        throw new Error(kind + " chain proxy script requires file-stage text input");
    }
    if (input.file.kind && input.file.kind !== kind) throw new Error("文件类型必须是 " + kind);
}
