/*
 * Sandrone 规范化节点序号重编脚本
 *
 * 用途：消费 normalize-nodes.js 的模板输出，作为 nodes-stage 的 script processor
 * 使用。在保持节点当前顺序不变、不增删节点、不修改连接参数的前提下，重新计算
 * 每个地区的 index。当前顺序可以来自任意上游处理。
 *
 * 脚本使用与 normalize-nodes.js 相同的 template 和 separator 定位 index，并以
 * region_code、flag、region 或 region_en 作为地区分组依据。index 可以位于模板中
 * 的任意位置；其他名称内容原样保留。无法匹配模板的节点保持原样。
 *
 * 常用参数（通过 ProcessorSpec.params.args 传入）：
 *   index_width=2               序号宽度，范围 1 到 4
 *   separator=" "                必须与 normalize-nodes.js 的 separator 相同
 *   template=...                 必须与 normalize-nodes.js 的 template 相同
 *   exclude_regex=[]            跳过重编号并原样保留，字符串或字符串数组
 *
 * 请将本脚本放在所有可能改变节点顺序的处理之后。
 */

var DEFAULT_TEMPLATE = "{prefix}{separator}{airport}{separator}{flag}{separator}{region}{separator}{index}{separator}{entry}{separator}{city}{separator}{line}{separator}{features}{separator}{multiplier}{separator}{protocol}";

var TEMPLATE_VARIABLES = {
    prefix: true, airport: true, flag: true, region: true, region_code: true,
    region_en: true, index: true, entry: true, city: true, line: true,
    features: true, multiplier: true, protocol: true, protocol_base: true,
    security: true, transport: true, flow: true, ip_stack: true, original: true,
    separator: true
};

var REGION_GROUP_VARIABLES = ["region_code", "flag", "region", "region_en"];
var INDEX_MARKER = "__SANDRONE_REINDEX_VALUE__";

function main(input, api) {
    input = input || {};
    var args = input.args || {};
    var indexWidth = readInteger(args.index_width, 2, 1, 4, "index_width");
    var separator = readString(args.separator, " ");
    if (!separator) throw new Error("separator 不能为空");
    var template = readTemplate(args.template, DEFAULT_TEMPLATE, "template");
    var matcher = buildTemplateMatcher(template, separator);
    var excludePatterns = readPatterns(args.exclude_regex, "exclude_regex");
    var nodes = Array.isArray(input.nodes) ? input.nodes : [];
    var indexes = Object.create(null);
    var reindexed = 0;
    var excluded = 0;
    var unmatched = 0;

    for (var index = 0; index < nodes.length; index += 1) {
        var node = nodes[index];
        if (!node || typeof node !== "object") continue;

        var name = node.name === undefined || node.name === null ? "" : String(node.name);
        if (matchesAny(name, excludePatterns)) {
            excluded += 1;
            continue;
        }
        var parsed = parseNormalizedName(name, matcher);
        if (!parsed) {
            unmatched += 1;
            continue;
        }

        indexes[parsed.regionCode] = (indexes[parsed.regionCode] || 0) + 1;
        node.name = parsed.beforeIndex + padNumber(indexes[parsed.regionCode], indexWidth) + parsed.afterIndex;
        reindexed += 1;
    }

    if (api && typeof api.warn === "function" && unmatched > 0) {
        api.warn({
            code: "node_reindex_template_unmatched",
            message: "kept nodes that did not match the normalize template: " + unmatched
        });
    }
    if (api && typeof api.log === "function") {
        api.log("reindexed", reindexed, "normalized nodes by region; excluded", excluded, "nodes");
    }
    return input;
}

function buildTemplateMatcher(template, separator) {
    var variables = templateVariableNames(template);
    if (countValue(variables, "index") !== 1) {
        throw new Error("template 必须包含且只能包含一个 {index}");
    }
    var groupVariables = [];
    for (var index = 0; index < REGION_GROUP_VARIABLES.length; index += 1) {
        if (variables.indexOf(REGION_GROUP_VARIABLES[index]) !== -1) groupVariables.push(REGION_GROUP_VARIABLES[index]);
    }
    if (groupVariables.length === 0) {
        throw new Error("template 必须包含 region_code / flag / region / region_en 之一");
    }

    var chunks = template.split("{separator}");
    var indexSource = buildTemplateSource(chunks, separator, "index", true);
    var markerStart = indexSource.indexOf(INDEX_MARKER);
    if (markerStart === -1 || indexSource.indexOf(INDEX_MARKER, markerStart + INDEX_MARKER.length) !== -1) {
        throw new Error("template 无法唯一定位 {index}");
    }
    var before = indexSource.slice(0, markerStart);
    var after = indexSource.slice(markerStart + INDEX_MARKER.length);
    var indexPattern = new RegExp("^(" + before + ")(\\d+)(" + after + ")$", "i");
    var groups = [];
    for (var groupIndex = 0; groupIndex < groupVariables.length; groupIndex += 1) {
        var variable = groupVariables[groupIndex];
        groups.push({
            variable: variable,
            pattern: new RegExp("^" + buildTemplateSource(chunks, separator, variable, false) + "$", "i")
        });
    }
    return {indexPattern: indexPattern, groups: groups};
}

function buildTemplateSource(chunks, separator, captureVariable, markIndex) {
    var separatorSource = escapeRegExp(separator);
    var captured = false;
    var source = "";
    for (var index = 0; index < chunks.length; index += 1) {
        var chunk = chunks[index];
        var chunkVariables = templateVariableNames(chunk);
        var chunkSource = compileTemplateChunk(chunk, captureVariable, markIndex, function () {
            if (captured) return false;
            captured = true;
            return true;
        });
        var required = chunkVariables.indexOf("index") !== -1 || (chunkVariables.length === 0 && cleanText(chunk) !== "");
        var trailingSeparator = index < chunks.length - 1 ? "(?:" + separatorSource + "|$)" : "";
        var segment = chunkSource + trailingSeparator;
        source += required ? segment : "(?:" + segment + ")?";
    }
    return source;
}

function compileTemplateChunk(chunk, captureVariable, markIndex, takeCapture) {
    var source = "";
    var cursor = 0;
    var pattern = /\{([^{}]+)\}/g;
    var match;
    while ((match = pattern.exec(chunk)) !== null) {
        source += escapeRegExp(chunk.slice(cursor, match.index));
        var variable = match[1];
        var valueSource = templateVariablePattern(variable);
        if (markIndex && variable === "index") source += INDEX_MARKER;
        else if (!markIndex && variable === captureVariable && takeCapture()) source += "(" + valueSource + ")";
        else source += "(?:" + valueSource + ")";
        cursor = match.index + match[0].length;
    }
    source += escapeRegExp(chunk.slice(cursor));
    return source;
}

function templateVariablePattern(variable) {
    if (variable === "index") return "\\d+";
    if (variable === "flag") return "\\uD83C[\\uDDE6-\\uDDFF]\\uD83C[\\uDDE6-\\uDDFF]";
    if (variable === "region_code") return "[A-Za-z]{2}";
    return ".+?";
}

function templateVariableNames(template) {
    var variables = [];
    String(template).replace(/\{([^{}]+)\}/g, function (_, key) {
        if (key !== "separator") variables.push(key);
        return _;
    });
    return variables;
}

function countValue(values, expected) {
    var count = 0;
    for (var index = 0; index < values.length; index += 1) if (values[index] === expected) count += 1;
    return count;
}

function parseNormalizedName(name, matcher) {
    var indexMatch = matcher.indexPattern.exec(String(name));
    if (!indexMatch) return null;
    var regionCode = "";
    for (var index = 0; index < matcher.groups.length; index += 1) {
        var group = matcher.groups[index];
        var groupMatch = group.pattern.exec(String(name));
        if (!groupMatch || !groupMatch[1]) continue;
        regionCode = regionGroupKey(group.variable, groupMatch[1]);
        if (regionCode) break;
    }
    if (!regionCode) return null;
    return {beforeIndex: indexMatch[1], afterIndex: indexMatch[3], regionCode: regionCode};
}

function regionGroupKey(variable, value) {
    value = cleanText(value);
    if (!value) return "";
    if (variable === "flag") return regionCodeFromFlag(value);
    if (variable === "region_code") return "code:" + value.toUpperCase();
    return variable + ":" + value.toLocaleLowerCase();
}

function regionCodeFromFlag(name) {
    var value = cleanText(name);
    if (value.length !== 4 || value.charCodeAt(0) !== 0xD83C || value.charCodeAt(2) !== 0xD83C) return "";
    var first = value.charCodeAt(1);
    var second = value.charCodeAt(3);
    if (first < 0xDDE6 || first > 0xDDFF || second < 0xDDE6 || second > 0xDDFF) return "";
    return String.fromCharCode(65 + first - 0xDDE6, 65 + second - 0xDDE6);
}

function readTemplate(value, fallback, name) {
    var template = value === undefined || value === null || value === "" ? fallback : String(value);
    template.replace(/\{([^{}]+)\}/g, function (_, key) {
        if (!TEMPLATE_VARIABLES[key]) throw new Error(name + " 包含未知变量: " + key);
        return _;
    });
    return template;
}

function escapeRegExp(value) {
    return String(value).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function readString(value, fallback) {
    return value === undefined || value === null ? fallback : String(value);
}

function readPatterns(value, name) {
    if (value === undefined || value === null || value === "") return [];
    var values = value;
    if (typeof value === "string" && value.trim().charAt(0) === "[") {
        try { values = JSON.parse(value); } catch (error) { throw new Error(name + " 的 JSON 数组无效: " + error.message); }
    }
    if (!Array.isArray(values)) values = [values];
    var patterns = [];
    for (var index = 0; index < values.length; index += 1) {
        if (typeof values[index] !== "string" || values[index] === "") {
            throw new Error(name + " 必须是非空字符串或字符串数组");
        }
        try { patterns.push(new RegExp(values[index], "i")); }
        catch (error) { throw new Error(name + " 包含无效正则: " + error.message); }
    }
    return patterns;
}

function matchesAny(value, patterns) {
    for (var index = 0; index < patterns.length; index += 1) {
        if (patterns[index].test(value)) return true;
    }
    return false;
}

function readInteger(value, fallback, minimum, maximum, name) {
    if (value === undefined || value === null || value === "") return fallback;
    var parsed = Number(value);
    if (!isFinite(parsed) || Math.floor(parsed) !== parsed || parsed < minimum || parsed > maximum) {
        throw new Error(name + " 必须是 " + minimum + " 到 " + maximum + " 的整数");
    }
    return parsed;
}

function cleanText(value) {
    if (value === undefined || value === null) return "";
    return String(value).replace(/[\u0000-\u001f\u007f]/g, " ").replace(/\s+/g, " ").trim();
}

function padNumber(value, width) {
    var output = String(value);
    while (output.length < width) output = "0" + output;
    return output;
}
