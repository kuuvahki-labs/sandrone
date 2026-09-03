/*
 * Sandrone Mihomo 链式代理文件脚本
 *
 * 作为 file-stage script processor 使用，直接修改最终 Mihomo YAML：
 *   1. 按 landing_pattern 识别落地节点并写入 dialer-proxy；
 *   2. 生成“前置代理”和“落地节点”两个 select 组。
 *
 * ProcessorSpec.params.args：
 *   landing_pattern   必填，落地节点名称正则（忽略大小写）
 *   front_proxies     可选，前置节点/代理组数组；省略时使用全部非落地节点
 *   front_group       可选，默认“前置代理”
 *   landing_group     可选，默认“落地节点”
 *
 * front_proxies 也可传 JSON 数组字符串，例如：
 *   ["香港节点", "日本节点"]
 *
 * 如需在其他组或 rules 中选择链式出口，引用 landing_group 即可。
 */

function main(input, api) {
    requireFile(input, "mihomo");
    var args = input.args || {};
    var matcher = readPattern(args.landing_pattern);
    var frontGroup = readName(args.front_group, "前置代理");
    var landingGroup = readName(args.landing_group, "落地节点");
    if (frontGroup === landingGroup) throw new Error("前置组和落地组不能同名");

    var doc = api.yaml.parse(input.file.content);
    var proxies = doc && Array.isArray(doc.proxies) ? doc.proxies : [];
    var groups = doc && Array.isArray(doc["proxy-groups"]) ? doc["proxy-groups"] : [];
    var landing = [];
    var ordinary = [];

    proxies.forEach(function(proxy) {
        if (!proxy || typeof proxy.name !== "string") return;
        if (matcher.test(proxy.name)) {
            proxy["dialer-proxy"] = frontGroup;
            landing.push(proxy.name);
        } else {
            ordinary.push(proxy.name);
        }
    });
    if (landing.length === 0) {
        api.warn({code: "chain_proxy_no_landing_nodes", message: "landing_pattern 未匹配任何 Mihomo 节点，文件保持不变"});
        return input;
    }

    var front = readStringList(args.front_proxies, ordinary);
    if (front.length === 0) throw new Error("前置代理组不能为空");
    var landingSet = toSet(landing);
    front.forEach(function(name) {
        if (name === frontGroup || name === landingGroup || landingSet[name]) {
            throw new Error("前置代理不能引用链式组或落地节点: " + name);
        }
        var group = findBy(groups, "name", name);
        if (!group) return;
        if (Array.isArray(group.proxies)) {
            group.proxies = group.proxies.filter(function(member) { return !landingSet[member]; });
        }
        if (group["include-all"] === true || group["include-all-proxies"] === true) {
            appendLandingExclusion(group, landing);
        }
    });

    groups = upsert(groups, "name", frontGroup, {name: frontGroup, type: "select", proxies: front});
    groups = upsert(groups, "name", landingGroup, {name: landingGroup, type: "select", proxies: landing});
    doc["proxy-groups"] = groups;
    input.file.content = api.yaml.stringify(doc);
    return input;
}

function appendLandingExclusion(group, landing) {
    var pattern = "^(?:" + landing.map(escapeRegExp).join("|") + ")$";
    var current = typeof group["exclude-filter"] === "string" ? group["exclude-filter"] : "";
    if (current.indexOf(pattern) === -1) group["exclude-filter"] = current ? current + "|" + pattern : pattern;
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
    if (!value) throw new Error("代理组名不能为空");
    return value;
}

function toSet(values) {
    var result = Object.create(null);
    values.forEach(function(value) { result[value] = true; });
    return result;
}

function escapeRegExp(value) {
    return String(value).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function requireFile(input, kind) {
    if (!input || input.stage !== "file" || !input.file || typeof input.file.content !== "string") {
        throw new Error(kind + " chain proxy script requires file-stage text input");
    }
    if (input.file.kind && input.file.kind !== kind) throw new Error("文件类型必须是 " + kind);
}
