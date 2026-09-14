/*
 * Sandrone Mihomo / sing-box / Shadowrocket 订阅筛选分组文件脚本
 *
 * 作为 file-stage script processor 使用，直接修改最终配置：
 *   1. 新增一个通过关键字或正则筛选订阅节点的 select 组；
 *   2. 把新分组插入 target_group 的显式成员列表首位。
 *
 * ProcessorSpec.params.args：
 *   filter         必填，节点名筛选表达式；普通关键字可直接填写
 *   target_group   必填，需要插入新分组的已有分组名
 *   group_name     可选，新分组名，默认“订阅筛选”
 *
 * 示例：
 *   args:
 *     filter: "(?i)premium|iplc"
 *     target_group: "🚀 节点选择"
 *     group_name: "精品节点"
 *
 * Mihomo 使用 include-all-proxies + filter；sing-box 从最终配置中的代理
 * outbound / endpoint 计算当前成员；Shadowrocket 使用 policy-regex-filter。
 * Shadowrocket 的 filter 不能包含逗号或换行。
 * 脚本可重复执行：同名筛选组会被更新，目标组中只保留一个首位引用。
 */

var SCRIPT_NAME = "subscription-filter-group.js";

var ADAPTERS = {
    mihomo: {apply: applyMihomo},
    "sing-box": {apply: applySingBox},
    shadowrocket: {apply: applyShadowrocket}
};

function main(input, api) {
    requireFile(input);
    var adapter = ADAPTERS[input.file.kind];
    if (!adapter) {
        throw new Error("文件类型必须是 mihomo、sing-box 或 shadowrocket");
    }

    var args = readArgs(input.args || {}, api);
    var content = applyOperation(input.file.content, args, api, adapter);
    if (content !== undefined) input.file.content = content;
    return input;
}

function readArgs(args, api) {
    var filter = readRequiredString(args.filter, "filter");
    var targetGroupName = readRequiredString(args.target_group, "target_group");
    var groupName = readOptionalString(args.group_name, "订阅筛选");
    if (groupName === targetGroupName) throw new Error("group_name 不能与 target_group 相同");
    return {filter: filter, targetGroupName: targetGroupName, groupName: groupName};
}

function applyOperation(content, args, api, adapter) {
    return adapter.apply(content, args, api);
}

function applySingBox(content, args, api) {
    var doc = api.json.parse(content);
    var filter = args.filter;
    var targetGroupName = args.targetGroupName;
    var groupName = args.groupName;
    if (!isObject(doc) || !Array.isArray(doc.outbounds)) {
        throw new Error("sing-box 文件根节点必须包含 outbounds 数组");
    }
    if (doc.endpoints !== undefined && !Array.isArray(doc.endpoints)) {
        throw new Error("sing-box endpoints 必须是数组");
    }

    var targetGroup = findSingBoxOutbound(doc.outbounds, targetGroupName);
    if (!targetGroup) throw new Error("未找到 target_group: " + targetGroupName);
    if (!isSingBoxGroup(targetGroup) || !isStringArray(targetGroup.outbounds)) {
        throw new Error("target_group 必须使用显式 outbounds 列表: " + targetGroupName);
    }

    var groupCollisions = doc.outbounds.filter(function(outbound) {
        return isObject(outbound) && outbound.tag === groupName;
    });
    if (groupCollisions.some(function(outbound) { return !isSingBoxGroup(outbound); })) {
        throw new Error("group_name 与非分组 outbound 冲突: " + groupName);
    }
    if ((doc.endpoints || []).some(function(endpoint) {
        return isObject(endpoint) && endpoint.tag === groupName;
    })) {
        throw new Error("group_name 与 endpoint 冲突: " + groupName);
    }

    var matcher = compilePattern(filter);
    var candidates = doc.outbounds.concat(doc.endpoints || []).filter(isSingBoxProxyTarget).map(function(outbound) {
        return outbound.tag;
    });
    var members = uniqueStrings(candidates.filter(function(tag) { return matcher.test(tag); }));

    if (members.length === 0) {
        api.warn({
            code: "subscription_filter_group_empty",
            message: "filter 未匹配任何 sing-box 代理 outbound 或 endpoint，已跳过分组: " + groupName
        });
        return undefined;
    }

    targetGroup.outbounds = [groupName].concat(targetGroup.outbounds.filter(function(member) {
        return member !== groupName;
    }));
    doc.outbounds = upsertSingBoxGroup(doc.outbounds, groupName, {
        type: "selector",
        tag: groupName,
        outbounds: members
    });
    return api.json.stringify(doc);
}

function applyMihomo(content, args, api) {
    var doc = api.yaml.parse(content);
    var filter = args.filter;
    var targetGroupName = args.targetGroupName;
    var groupName = args.groupName;
    if (!doc || typeof doc !== "object" || Array.isArray(doc)) {
        throw new Error("Mihomo 文件根节点必须是 YAML 对象");
    }
    var groups = Array.isArray(doc["proxy-groups"]) ? doc["proxy-groups"] : [];
    var targetGroup = findMihomoGroup(groups, targetGroupName);
    if (!targetGroup) throw new Error("未找到 target_group: " + targetGroupName);
    if (!Array.isArray(targetGroup.proxies)) {
        throw new Error("target_group 必须使用显式 proxies 列表: " + targetGroupName);
    }

    targetGroup.proxies = [groupName].concat(targetGroup.proxies.filter(function(member) {
        return member !== groupName;
    }));
    doc["proxy-groups"] = upsertMihomoGroup(groups, groupName, {
        name: groupName,
        type: "select",
        "include-all-proxies": true,
        filter: filter
    });
    return api.yaml.stringify(doc);
}

function applyShadowrocket(content, args, api) {
    var filter = args.filter;
    var targetGroupName = args.targetGroupName;
    var groupName = args.groupName;
    if (/[,\r\n]/.test(filter)) throw new Error("Shadowrocket filter 不能包含逗号或换行");
    requireShadowrocketName(targetGroupName, "target_group");
    requireShadowrocketName(groupName, "group_name");

    var doc = api.ini.parse(content);
    var sections = doc && Array.isArray(doc.sections) ? doc.sections : [];
    var targetSection = null;
    var targetLine = null;
    for (var sectionIndex = 0; sectionIndex < sections.length && !targetLine; sectionIndex += 1) {
        var section = sections[sectionIndex];
        if (!section || section.name !== "Proxy Group" || !Array.isArray(section.lines)) continue;
        for (var lineIndex = 0; lineIndex < section.lines.length; lineIndex += 1) {
            var parsed = parseShadowrocketGroup(section.lines[lineIndex]);
            if (parsed && parsed.name === targetGroupName) {
                targetSection = section;
                targetLine = parsed;
                break;
            }
        }
    }
    if (!targetLine) throw new Error("未找到 target_group: " + targetGroupName);
    if (hasShadowrocketDynamicSource(targetLine.values) || !hasShadowrocketMember(targetLine.values)) {
        throw new Error("target_group 必须使用显式成员列表: " + targetGroupName);
    }

    var remaining = targetLine.values.slice(1).filter(function(member) {
        return member !== groupName;
    });
    replaceShadowrocketGroupLine(targetSection, targetGroupName,
        targetGroupName + " = " + [targetLine.values[0], groupName].concat(remaining).join(","));

    sections.forEach(function(section) {
        if (!section || section.name !== "Proxy Group" || !Array.isArray(section.lines)) return;
        section.lines = section.lines.filter(function(line) {
            var parsed = parseShadowrocketGroup(line);
            return !parsed || parsed.name !== groupName;
        });
    });
    var targetIndex = findShadowrocketGroupLine(targetSection.lines, targetGroupName);
    targetSection.lines.splice(targetIndex + 1, 0,
        groupName + " = select,policy-regex-filter=" + filter);
    return api.ini.stringify(doc);
}

function upsertMihomoGroup(groups, name, replacement) {
    var found = false;
    return groups.map(function(group) {
        if (!group || group.name !== name) return group;
        if (found) return null;
        found = true;
        return replacement;
    }).filter(function(group) {
        return group !== null;
    }).concat(found ? [] : [replacement]);
}

function findMihomoGroup(groups, name) {
    for (var index = 0; index < groups.length; index += 1) {
        if (groups[index] && groups[index].name === name) return groups[index];
    }
    return null;
}

function upsertSingBoxGroup(outbounds, tag, replacement) {
    var found = false;
    return outbounds.map(function(outbound) {
        if (!isObject(outbound) || outbound.tag !== tag) return outbound;
        if (found) return null;
        found = true;
        return replacement;
    }).filter(function(outbound) {
        return outbound !== null;
    }).concat(found ? [] : [replacement]);
}

function findSingBoxOutbound(outbounds, tag) {
    for (var index = 0; index < outbounds.length; index += 1) {
        if (isObject(outbounds[index]) && outbounds[index].tag === tag) return outbounds[index];
    }
    return null;
}

function isSingBoxGroup(outbound) {
    return isObject(outbound) && (outbound.type === "selector" || outbound.type === "urltest");
}

function isSingBoxProxyTarget(outbound) {
    if (!isObject(outbound) || typeof outbound.type !== "string" ||
        typeof outbound.tag !== "string" || !outbound.tag.trim()) return false;
    return outbound.type !== "selector" && outbound.type !== "urltest" &&
        outbound.type !== "direct" && outbound.type !== "block" && outbound.type !== "dns";
}

function compilePattern(value) {
    var insensitive = value.indexOf("(?i)") === 0;
    var pattern = insensitive ? value.slice(4) : value;
    if (!pattern.trim()) throw new Error("filter 必须是非空筛选表达式");
    try {
        return new RegExp(pattern, insensitive ? "i" : "");
    } catch (error) {
        throw new Error("filter 不是有效正则表达式: " + error.message);
    }
}

function uniqueStrings(values) {
    return values.filter(function(value, index) { return values.indexOf(value) === index; });
}

function isStringArray(value) {
    return Array.isArray(value) && value.every(function(item) {
        return typeof item === "string" && item !== "";
    });
}

function parseShadowrocketGroup(line) {
    if (typeof line !== "string") return null;
    var trimmed = line.trim();
    if (!trimmed || trimmed.charAt(0) === "#" || trimmed.charAt(0) === ";") return null;
    var separator = line.indexOf("=");
    if (separator < 0) return null;
    var name = line.slice(0, separator).trim();
    var values = line.slice(separator + 1).split(",").map(function(value) {
        return value.trim();
    });
    if (!name || values.length === 0 || !values[0]) return null;
    return {name: name, values: values};
}

function hasShadowrocketDynamicSource(values) {
    for (var index = 1; index < values.length; index += 1) {
        if (values[index] === "use=true" || values[index].indexOf("policy-regex-filter=") === 0) return true;
    }
    return false;
}

function hasShadowrocketMember(values) {
    for (var index = 1; index < values.length; index += 1) {
        if (values[index] && values[index].indexOf("=") < 0) return true;
    }
    return false;
}

function replaceShadowrocketGroupLine(section, name, replacement) {
    var index = findShadowrocketGroupLine(section.lines, name);
    if (index < 0) throw new Error("未找到 target_group: " + name);
    section.lines[index] = replacement;
}

function findShadowrocketGroupLine(lines, name) {
    for (var index = 0; index < lines.length; index += 1) {
        var parsed = parseShadowrocketGroup(lines[index]);
        if (parsed && parsed.name === name) return index;
    }
    return -1;
}

function requireShadowrocketName(value, name) {
    if (/[\r\n=,]/.test(value) || /^[#;\[]/.test(value)) {
        throw new Error(name + " 不是合法的 Shadowrocket 分组名");
    }
}

function readRequiredString(value, name) {
    if (typeof value !== "string" || !value.trim()) throw new Error(name + " 必须是非空字符串");
    return value.trim();
}

function readOptionalString(value, fallback) {
    if (value === undefined || value === null || value === "") return fallback;
    if (typeof value !== "string" || !value.trim()) throw new Error("group_name 必须是非空字符串");
    return value.trim();
}

function requireFile(input) {
    if (!input || input.stage !== "file" || !input.file || typeof input.file.content !== "string") {
        throw new Error(SCRIPT_NAME + " requires file-stage text input");
    }
}

function isObject(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
