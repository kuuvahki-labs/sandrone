# JavaScript 脚本 API

Sandrone 的 `script` processor 在受限的 JavaScript 运行时中处理节点或完整文件。本页是脚本参数、输入 envelope、注入 API、返回值和失败边界的现行契约。processor 的阶段、声明顺序及其他内建 processor 见 [Processors](processors.md)。

## Processor 参数

`script` 同时注册在 `nodes` 和 `file` 阶段，因此 `stage` 必须显式填写：

```yaml
processors:
  - type: script
    stage: nodes
    params:
      source:
        type: inline
        content: |
          function main(input, api) {
            input.nodes = input.nodes.filter((node) => node.type !== "http");
            return input;
          }
      args:
        environment: production
      timeout_ms: 1000
```

`params` 使用严格字段解码，未知字段会得到 `processor_config_invalid`。

| 字段 | 类型 | 当前语义 |
| --- | --- | --- |
| `source` | object | 必填且只能指定一个脚本来源，见下一节 |
| `engine` | string | 省略时为 `js`；当前只接受 `js` |
| `args` | object | processor 级参数，合并后作为 `input.args` |
| `timeout_ms` | integer | 大于 `0` 时覆盖执行超时；省略、`0` 或负数继承项目 `script_defaults.timeout_ms` |
| `id` | string | inline 脚本的诊断标识；省略时为 `<inline>` |
| `permissions` | object | 保留配置；当前不授予原生宿主能力 |

配置结构中虽保留 `permissions` 字段，但它不是授予原生文件系统、子进程、环境变量或通用网络访问的接口。脚本能产生的外部作用仅限本页列出的受控 API。
在 MCP wire 上，script processor 的 `params` 与其中的 `permissions` 都是
JSON object，不是字符串。Agent 可通过
`sandrone://schemas/script-api/v1` 与对应的
`sandrone://schemas/processors/{stage}/script` resource 读取机器可校验的当前
契约；resource catalog 见 [MCP 参考](mcp.md#resources-与-schema-templates)。

## 脚本来源

### `inline`

`content` 必须是非空脚本文本：

```yaml
source:
  type: inline
  content: |
    function main(input) {
      return input;
    }
```

### `file`

`name` 是 Sandrone 中已登记的文件资源名，不是宿主文件系统路径，也不能写成内部存储键（例如 `files/example.js`）：

```yaml
source:
  type: file
  name: normalize.js
```

加载器以 `render` 模式执行该文件资源，并把最终正文当作 JavaScript。也就是说，
typed 编译和该资源自身的 file-stage processors 都会生效；这些 processors 使用
各自配置的参数，当前 script processor 的参数和请求参数不会传入脚本文件渲染。
文件资源自身只能使用支持的 inline 或 remote FileSource；非法、绝对、越界或
写成内部 `files/` key 的资源名会被拒绝。

### `remote`

`remote.url` 必填。获取过程走 Sandrone 的受控远程 fetcher、运行时默认值和缓存边界，不向脚本暴露通用 HTTP 客户端：

```yaml
source:
  type: remote
  remote:
    url: https://example.com/scripts/normalize.js
    timeout_ms: 5000
    cache_ttl_seconds: 300
  sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

`sha256` 可选；出现时必须与抓取内容的 SHA-256 十六进制摘要一致，否则得到 `processor_config_invalid`。远程输入本身的字段与安全边界见 [FileSpec](file-spec.md)。

## 入口函数

脚本必须在顶层定义可调用的：

```js
function main(input, api) {
  return input;
}
```

- 脚本顶层代码先执行，然后调用 `main(input, api)`。
- `input` 是普通 JavaScript 对象，可以在本次调用内修改。
- `api` 是受控注入对象；同一对象也以全局变量 `api` 提供。
- `main` 是同步接口；返回的 Promise 不会被等待。
- 缺少可调用的 `main`、顶层求值失败或 `main` 抛出异常均为 `script_runtime`。

## 通用 envelope

两个阶段都接收版本为 `1` 的 envelope：

```js
{
  version: 1,
  stage: "nodes",          // "nodes" 或 "file"
  target: "...",           // 可能省略
  render_options: {},
  context: {},
  request: {},
  response: {},
  args: {},
  nodes: [],
  file: {},
  parts: [],
  warnings: []
}
```

JSON 中带 `omitempty` 的成员在无值时可能不存在。脚本应对 `args`、`nodes`、`file`、`parts` 和 `warnings` 做缺省保护，不应依赖空数组或空对象必然出现。

| 字段 | 语义 |
| --- | --- |
| `version` | envelope 版本，当前为 `1` |
| `stage` | 当前阶段：`nodes` 或 `file` |
| `target` | 当前目标格式或调用方给出的 target；可能为空 |
| `render_options` | 渲染选项对象；当前公开字段为可选 `format` |
| `context` | 节点阶段的输入名、依赖、来源与 metadata；文件阶段当前为空 |
| `request` | 请求信息：可选 `trace_id`、字符串 `args` 与字符串 `meta` |
| `response` | 响应信息：可选 `headers` 与 `status`；当前处理入口通常为空 |
| `args` | 合并后的脚本参数，规则见下文 |
| `warnings` | processor 级 warning 数组 |

### `nodes` 阶段

`input.nodes` 是当前 processor 链传入的节点数组。节点字段使用 Sandrone `NodeIR` 的 JSON 名称，包括：

- 身份与端点：`id`、`name`、`type`、`server`、`port`；
- 认证与协议：`username`、`password`、`uuid`、`cipher`、`alter_id`、`flow`、`encryption`、`token`；
- 网络与扩展选项：`network`、`packet_encoding`、`plugin`、`plugin_options`、`headers`、`path`、`tls`、`dialer`、`transport`、`multiplex`、`udp_over_tcp`；
- 协议专属对象：`shadowsocksr`、`snell`、`anytls`、`hysteria`、`tuic`、`mieru`、`wireguard`；
- 诊断与来源：`tags`、`meta`、`raw`、`lossy`、`warnings`、`source_format`；
- `ext`：脚本自定义扩展字段。

返回转换时，每个节点都必须保留非空 `name` 和 `type`。`ext` 的每个键会存入节点的 `raw["script.ext."+key]`，并产生一个 `script_ext_field` warning；任意值无法 JSON 编码时转换失败。

订阅 preview 会给输入节点附加一个不属于上述 JSON 字段的运行时来源标记。
原地修改节点、筛选数组、排序以及用 `{...node}` 展开已有节点都会保留该标记；
它不会出现在 `Object.keys(node)`、`JSON.stringify(node)`、NodeIR JSON 或节点详情中。
如果脚本只从上述公开字段手工构造一个新对象，preview 会把它视为新节点，同时把
未返回的原节点视为已删除，不会再按连接字段猜测两者是同一个节点。

### `file` 阶段

`input.file` 是当前完整文件：

```js
{
  name: "config.yaml",
  kind: "mihomo",
  media_type: "application/yaml",
  encoding: "utf-8",
  content: "...",
  meta: {},
  warnings: []
}
```

`content` 始终以字符串交给脚本，返回后转回字节。返回的 `name`、`kind` 和 `content` 会覆盖当前文件；非空 `media_type`、非空 `encoding`、非 `null` 的 `meta` 与 `warnings` 才覆盖原值。

## `input.args` 合并

`params.args` 先作为基值，请求携带的 `request.args` 再按同名键覆盖。请求参数在 HTTP/CLI/内部调用边界上是字符串；processor 级值可以是任意可 JSON 编码值。

如果两个来源都为空，`input.args` 可能不存在：

```js
const prefix = (input.args && input.args.prefix) || "";
```

`params.args` 只参与当前脚本的 `input.args` 合并，不会传给 file-backed source
的文件渲染流程。

`api.subscription.produce` 与 `api.file.content` 的 `options.args` 只接受字符串
键值，并作为子调用的完整参数集。省略 `options.args` 表示子调用无参数；父文件
请求参数和当前脚本 `input.args` 都不会隐式继承。

## 注入 API

### 纯计算与诊断

| API | 返回与行为 |
| --- | --- |
| `api.log(...values)` | 接收并格式化本次运行的诊断文本，返回 `undefined`；当前输出与 report 不公开这些日志 |
| `api.warn(warning)` | 追加 warning，返回 `undefined`；缺少 `code` 时补为 `script_warning` |
| `api.yaml.parse(text)` | 将 YAML 解码为普通 JS 值；无参数返回 `undefined` |
| `api.yaml.stringify(value)` | 返回 YAML 字符串；无参数返回空字符串 |
| `api.json.parse(text)` | 返回普通 JS 值；无参数返回 `undefined` |
| `api.json.stringify(value)` | 返回紧凑 JSON 字符串；无参数返回空字符串 |
| `api.ini.parse(text)` | 返回保留顺序与重复 section 的 INI 文档；无参数返回 `undefined` |
| `api.ini.stringify(document)` | 严格校验并返回 INI 字符串；无参数返回空字符串 |
| `api.ini.override(base, patch)` | 按 `ini_override` 的 section 规则覆盖正文；两个参数都必需 |
| `api.base64.encode(text)` | 返回标准、有 padding 的 Base64；无参数返回空字符串 |
| `api.base64.decode(text)` | 解码标准 Base64 并返回字符串；无参数返回空字符串 |
| `api.hash.sha256(text)` | 返回小写十六进制 SHA-256；无参数返回空字符串 |

`api.warn` 接受 warning 的当前字段：`code`、`message`、`node`、`node_index`、`node_context`、`field`、`source`、`target`。无法转换为该结构的值不会产生 warning。

### INI 文档模型

`api.ini.parse` 不会把 INI 折叠为 section/key object。返回值保留 physical
section 的顺序、重复 section、注释、空行和 Shadowrocket 的非 assignment
记录：

```js
{
  bom: false,
  newline: "\n",
  trailing_newline: true,
  preamble: ["# generated"],
  sections: [
    {name: "General", lines: ["dns-server = 1.1.1.1"]},
    {name: "Rule", lines: ["FINAL,Proxy"]}
  ]
}
```

`newline` 只能是 `"\n"` 或 `"\r\n"`；`preamble`、`sections` 和每个
section 的 `lines` 都必须是数组。section 名必须是已去除首尾空白的非空名称，
且不能包含方括号或换行。单行不能包含 CR/LF，也不能伪装成新的 section
header；未知字段同样会使 `api.ini.stringify` 失败。输出统一使用 `newline`
并规范化为 `[name]` header，因此混合换行和原 header 外观不会逐字节保留。

`api.ini.override(base, patch)` 直接使用 file-stage
`merge(mode: ini_override)` 的同一套覆盖实现，包括 section 合并、前插、追加、
替换和删除语义；具体 patch 写法见 [Processors](processors.md#merge)。

### `api.probe(nodes, options?)`

仅在 `nodes` 阶段提供。`nodes` 必须是原始 `input.nodes` 的逐项精确子集；不能借此探测脚本新建或已改写的任意目标。允许重复项的数量也不能超过输入。

`options` 可含：

```js
{
  layer: "protocol",
  method: "tcp_connect",
  core: "",
  url: "",
  ntp_server: "",
  expected_status: "",
  timeout_ms: 1000,
  attempts: 1,
  concurrency: 4,
  cache_ttl_seconds: 60,
  meta: { source: "script" }
}
```

返回对象含 `results` 与 `report`，形状与普通 probe 结果一致。probe report 中的 warnings 会追加到脚本输出 warning。调用服从脚本总超时；不可用、参数非法、后端错误或返回空结果会令脚本失败。

### `api.subscription.produce(name, options?)`

解析已登记订阅。`options` 可含 `target` 与字符串 `args`；`args` 是该子调用的
完整参数集，不继承父调用：

- 没有 `target` 时返回 `{kind: "nodes", nodes, report}`；
- 有 `target` 时用对应 renderer 返回 `{kind: "content", target, content, report}`；
- 返回对象的 `report` 描述该订阅的依赖、来源和 warning；在文件渲染上下文中，直接订阅还会记入外层文件的动态依赖。

空名称、未知订阅、递归解析失败或未知 target 会令脚本失败。

### `api.file.content(name, options?)`

读取另一个已登记文件的最终处理结果，返回内容字符串。`options.args` 是传给
该文件的完整字符串参数集，省略时不会继承父调用参数。该 API 只在文件渲染
上下文中可用，并记录动态文件依赖；依赖环得到 `file_dependency_cycle`。

## 返回值与转换

- 返回值必须是可解码为 envelope 的 JSON 对象，不能直接返回裸节点数组或裸 file 对象。通常应修改并返回整个 `input`；若返回部分对象，省略字段先变为零值，再由当前阶段选取其输出。
- 返回 `undefined`、`null` 或没有显式 `return` 时，保留调用前的 envelope；对 `input` 的普通修改不会回写，但 `api.warn` 产生的 warning 仍会追加。
- 返回值必须可 JSON 编码且能解码为 envelope；函数、循环引用或类型不兼容会得到 `script_runtime`。
- `nodes` 阶段只采用返回 envelope 的 `nodes` 与 `warnings`。
- `file` 阶段只采用返回 envelope 的 `file` 与 `warnings`；对其他顶层字段或 `parts` 的修改不构成输出。

## 超时、隔离与敏感信息

每次执行有一个总超时。未配置 processor 级正数 `timeout_ms` 时继承项目
`script_defaults.timeout_ms`，该项目默认值为 2000 ms。它覆盖顶层求值、`main`、`api.probe`
及执行期间的受控资源调用；超时返回 `script_timeout`。脚本 source 的加载和
编译发生在该执行时限之前，remote source 使用自己的抓取超时。调用方 context
取消也会中断执行，但不伪装成脚本自身超时。

运行时是 Go 内嵌 ECMAScript 引擎，不是 Node.js，也不是通用代码执行环境。
脚本没有 `require`、Node.js 模块、任意文件系统、子进程、环境变量或通用网络
API。即使填写保留的 `permissions` object，也不会出现这些能力。远程脚本抓取、
订阅生成、文件读取和节点探测只能通过 Sandrone 的受控边界进行。

envelope 可能包含代理密码、UUID、token、原始节点字段、请求 metadata 和来源诊断。脚本、`api.log`、`api.warn` 以及错误原因都不得当作脱敏边界；不要把秘密写入 warning、日志或可公开的错误响应。

错误码和 warning/report 的通用语义见 [错误与诊断](errors.md)。
