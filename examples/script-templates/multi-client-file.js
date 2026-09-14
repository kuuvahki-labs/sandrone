/*
 * Sandrone 多客户端 file-stage 脚本模板
 *
 * 复制本文件到 examples/scripts/<name>.js 后：
 *   1. 修改 SCRIPT_NAME；
 *   2. 在 ADAPTERS 中保留实际支持的客户端；
 *   3. 在 readArgs 中解析客户端无关的业务参数；
 *   4. 在各 apply 函数中解析、修改并序列化对应客户端正文。
 *
 * 每个成品脚本必须保持单文件自包含，不依赖 require、import 或构建步骤。
 */

var SCRIPT_NAME = "replace-me.js";

var ADAPTERS = {
    "mihomo": {apply: applyMihomo},
    "sing-box": {apply: applySingBox},
    "shadowrocket": {apply: applyShadowrocket}
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
    // TODO: 返回客户端无关的业务参数；需要 JSON 时使用 api.json.parse。
    return {};
}

function applyOperation(content, args, api, adapter) {
    return adapter.apply(content, args, api);
}

function applyMihomo(content, args, api) {
    var document = api.yaml.parse(content);
    // TODO: 修改 document；无需修改时返回 undefined。
    return api.yaml.stringify(document);
}

function applySingBox(content, args, api) {
    var document = api.json.parse(content);
    // TODO: 修改 document；无需修改时返回 undefined。
    return api.json.stringify(document);
}

function applyShadowrocket(content, args, api) {
    var document = api.ini.parse(content);
    // TODO: 修改 document；需要保留原始正文时也可以使用 api.ini.override。
    return api.ini.stringify(document);
}

function requireFile(input) {
    if (!input || input.stage !== "file" || !input.file ||
        typeof input.file.content !== "string") {
        throw new Error(SCRIPT_NAME + " requires file-stage text input");
    }
}
