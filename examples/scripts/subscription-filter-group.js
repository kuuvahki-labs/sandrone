/*
 * Sandrone Mihomo / Shadowrocket 订阅筛选分组文件脚本
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
 * Mihomo 使用 include-all-proxies + filter；Shadowrocket 使用
 * policy-regex-filter。Shadowrocket 的 filter 不能包含逗号或换行。
 * 脚本可重复执行：同名筛选组会被更新，目标组中只保留一个首位引用。
 */

function main(input, api) {
    requireSupportedFile(input);
    var args = input.args || {};
    var filter = readRequiredString(args.filter, "filter");
    var targetGroupName = readRequiredString(args.target_group, "target_group");
    var groupName = readOptionalString(args.group_name, "订阅筛选");
    if (groupName === targetGroupName) throw new Error("group_name 不能与 target_group 相同");

    if (input.file.kind === "shadowrocket") {
        applyShadowrocket(input, api, filter, targetGroupName, groupName);
    } else {
        applyMihomo(input, api, filter, targetGroupName, groupName);
    }
    return input;
}

function applyMihomo(input, api, filter, targetGroupName, groupName) {
    var doc = api.yaml.parse(input.file.content);
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
    input.file.content = api.yaml.stringify(doc);
}

function applyShadowrocket(input, api, filter, targetGroupName, groupName) {
    if (/[,\r\n]/.test(filter)) throw new Error("Shadowrocket filter 不能包含逗号或换行");
    requireShadowrocketName(targetGroupName, "target_group");
    requireShadowrocketName(groupName, "group_name");

    var doc = api.ini.parse(input.file.content);
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
    input.file.content = api.ini.stringify(doc);
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

function requireSupportedFile(input) {
    if (!input || input.stage !== "file" || !input.file || typeof input.file.content !== "string") {
        throw new Error("subscription filter group script requires file-stage text input");
    }
    if (input.file.kind !== "mihomo" && input.file.kind !== "shadowrocket") {
        throw new Error("文件类型必须是 mihomo 或 shadowrocket");
    }
}
