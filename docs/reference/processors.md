# Processors 参考

Processor 是显式声明、按顺序执行的转换步骤。本页定义当前 registry、参数和
失败语义。脚本的 envelope、注入 API 与 sandbox 见
[Scripting API](scripting-api.md)；错误对象字段见 [错误参考](errors.md)。

## ProcessorSpec

```yaml
type: rename
stage: nodes
name: optional-label
enabled: false
params:
  mode: prefix
  value: "HK-"
```

| 字段 | 契约 |
| --- | --- |
| `type` | 必填，必须是已注册 processor type。 |
| `stage` | `nodes` 或 `file`。仅当该 type 只注册在一个 stage 时可省略并推断。 |
| `name` | 可选声明名；不改变 type 或执行语义。 |
| `enabled` | 可选布尔值；省略或 `true` 时执行，`false` 时保留声明但跳过。 |
| `params` | object；每个内建 processor 都拒绝未知字段。 |

`script` 同时注册在 `nodes` 和 `file`，所以使用时必须显式写 `stage`。未知
type 返回 `processor_unknown`；stage 无法唯一推断或参数无效返回
`processor_config_invalid`。

在 MCP tool 的 JSON wire 上，`params` 直接是 object；调用方不应把它编码成
JSON 字符串。可发现的 processor 摘要与逐项 schema URI 见
[MCP resources](mcp.md#resources-与-schema-templates)，本页继续定义领域语义。

每次运行先跳过 `enabled: false` 的项，再选择当前 stage，并保留启用项在原数组中的相对顺序。关闭的项不会解析 stage、构建 processor 或执行参数校验；其完整声明仍保留在资源中，重新启用后恢复原位置。前一步输出
是后一步输入，warning 也按此顺序累积。file-stage `merge` 与 `script` 没有
特殊优先级：它们严格出现在声明位置。

`FileSpec.processors` 由文件流程作为 file stage 运行；其中声明为 `nodes` 的
项会被路由后跳过。文件引用的订阅节点要在订阅自身的 processor 链中处理；
parse/render 等入口则运行各自请求中的 nodes-stage 数组。

对 typed `FileSpec`，driver 先完成客户端配置编译，然后才运行 file-stage
链。例如下列顺序是“覆盖 YAML → 脚本 → 再覆盖 YAML”：

```yaml
processors:
  - type: merge
    stage: file
    params: {mode: yaml_override, content: "mode: global"}
  - type: script
    stage: file
    params:
      source: {type: file, name: postprocess.js}
  - type: merge
    stage: file
    params: {mode: yaml_override, content: "log-level: warning"}
```

## nodes stage

当前内建 type 为 `filter`、`dedup`、`rename`、`sort`、`quick_settings`、
`probe`、`script`。

### filter

只接受一个匹配规则：

- `action`：必填，`keep` 或 `drop`；
- `field`：`name`、`type` 或 `server`；
- `match: regex`：必须同时给出 `pattern`；
- `match: in`：必须同时给出非空 `values`。

`type` 的 `in` 比较会去空白并转小写；其余值保持大小写。输出保持原相对顺序。

### dedup

以下策略保留每个 key 首次出现的节点：

- `strategy: name`（缺省）：只使用节点名；
- `strategy: connection`：使用规范化 `NodeIR` 的完整连接语义；名称、标签、
  metadata、来源原文与诊断字段不参与判断；
- `strategy: fields`：按 `fields` 声明顺序组合 key，数组不能为空。

同名节点也可以选择保留并改名：

- `strategy: random_suffix`：不删除同名节点；保留首次出现的名称，后续同名节点
  追加 `-` 和 4 位随机数字，并避开当前批次已有名称。

`fields` 中有意义的当前字段为 `name`、`type`、`server`、`port`、`uuid`、
`password`、`username`、`cipher`。

### rename

一次 processor 对每个名称只做一轮，顺序固定为：

1. 按 `strip[]` 顺序删除所有匹配片段；
2. `trim: true` 时去除首尾空白；
3. 执行可选 `mode`。

`mode` 可为：

- `replace`：正则 `pattern` 替换为 `replacement`；
- `prefix`、`suffix`：使用非空 `value`；
- `template`：`value` 可含 `{name}`、`{type}`、`{server}`、
  `{source_format}`。

必须至少声明 `trim`、非空 `strip` 或 `mode` 之一。多轮改名应声明多个
`rename`，其数组顺序就是轮次顺序。

### sort

`by` 是逗号分隔的 key；`+` 表示升序，`-` 表示降序，缺省为 `+name`。
排序稳定，多个 key 从左到右比较。可比较字段与 `dedup.fields` 相同，值按
字符串比较。

### quick_settings

参数为 `udp`、`tfo`、`allow_insecure`、`vmess_aead`、`reuse`。每项只接受
`enabled`、`disabled`、`default`；省略等同 `default`，即保持节点值。

| 参数 | `enabled` | `disabled` |
| --- | --- | --- |
| `udp` | 明确设置 UDP relay 为 true | 明确设置为 false |
| `tfo` | 创建/更新 dialer 并开启 TFO | 已有 dialer 时关闭；不会只为关闭而创建 |
| `allow_insecure` | 开启 TLS 并允许跳过证书校验 | 已有 TLS 时关闭跳过校验 |
| `vmess_aead` | VMess `alter_id` 设为 0 | 仅保留已有非零 `alter_id`；无法构造 legacy 值时 warning |
| `reuse` | 按 Snell 版本开启 | 按 Snell 版本关闭；协议不支持或语义受限时 warning |

`vmess_aead` 只作用于 VMess，`reuse` 只作用于带 Snell 选项的节点。

### probe

`probe` 把当前节点批次交给 service probe runner。探测参数为：

`method`、`core`、`url`、`ntp_server`、`expected_status`、
`timeout_ms`、`attempts`、`concurrency`、`cache_ttl_seconds`。
`expected_status` 对 sing-box 和 Mihomo 使用相同语义：空值或 `*` 接受任意状态；
`204` 表示精确值；`200-299` 表示闭区间；`/` 或 `,` 可连接多个候选，例如
`200/204/301-303`。首尾空白和空候选会被忽略；每个值必须是 `0..65535` 的
无符号整数，最多 28 个候选。非法配置产生 `probe_invalid_target`，响应状态不匹配
则该节点探测失败。
除 `expected_status` 外，省略值由 probe service 使用当前项目运行默认值处理；
`timeout_ms`、`attempts`、`concurrency` 和 `cache_ttl_seconds` 的 `0` 与省略等价，
其中缓存默认值来自 `cache_defaults.probe_ttl_seconds`。
该 TTL 只在 processor 属于已保存 Subscription 的执行作用域时形成持久缓存；临时
diagnose、inline convert 和未保存草稿仍会执行探测，但不会读写持久缓存。
客户端不应把某次读取到的运行默认值写入新 processor，否则后续全局设置变化不会
再传递到该 processor。
method 只接受 `tcp_connect`、`udp_ntp` 和 `url_test`；默认 `url_test`。
`tcp_connect` 不使用 core，`udp_ntp` 当前使用 sing-box，`url_test` 支持
sing-box 和 Mihomo，省略 core 时默认 sing-box。

处理结果由以下参数决定：

- `fail_mode: keep`（缺省）：保留失败节点；
- `fail_mode: drop`：丢弃失败节点；
- `fail_mode: error`：遇到第一个失败结果即让整步失败；
- `annotate: true`：重写该节点所有 `probe.*` meta，写入 method、core、
  alive、duration、checked_at、error_code 等已有结果；
- `sort: duration`：存活节点在前并按延迟升序；失败节点在后，平局保持输入顺序。

执行 runner 前，processor 会检查当前批次的 `NodeIR.Name`。只要存在重名，整次
processor 就跳过探测，按原顺序原样返回全部节点，并只产生一条
`probe_skipped_duplicate_node_names` warning；warning 汇总重名组数和涉及的节点数，
不展开具体名称。此时不会改写 `probe.*` meta，`fail_mode`、`annotate` 和 `sort`
均不生效。preview 与其它执行这条 nodes processor 链的入口共享该行为；直接调用
probe service 不经过这项 processor 前置检查。

当前运行时完全没有 probe backend 时，processor 同样原样返回节点并继续后续步骤，
产生 `probe_skipped_backend_unavailable` warning；`fail_mode`、`annotate` 和 `sort`
均不生效。保存、导入或备份恢复不会删除或禁用该 processor，因此同一配置迁移回
支持 probe 的运行时后会恢复执行。直接 Probe API、CLI probe 和脚本 `api.probe`
属于显式探测调用，backend 不可用时仍返回 `probe_backend_unavailable`；指定了不可用
method/core 也继续返回对应错误，不按这条全局不可用规则跳过。
这份跳过 probe 的降级结果不会写入 subscription-snapshot；如果执行 processor 前已
命中由其他有能力运行时写入的兼容快照，则直接复用该快照。共享缓存的完整身份与
生产者/消费者规则见[存储与并发](../architecture/storage.md#缓存层)。

runner 返回的结果数必须与输入节点数相同。probe report warning 会并入 processor
warning；runner 错误直接终止链。使用 sing-box core 时，核心目标不能表达的节点
由 runner 在原位置返回 `probe_invalid_target`，而不是让可探测的同批节点失败；
因此 `fail_mode` 在 runner 返回后按节点结果执行。这个 node-level isolation 保证
目前不适用于 Mihomo，其 runner 错误仍会终止 processor 链。Hysteria v1 的
canonical 字段与单位边界见
[Hysteria v1 带宽规范化](capabilities.md#hysteria-v1-带宽规范化)，本页不重复定义。

### nodes script

`script` 的 `engine` 当前只能是 `js`，source 可为 `inline`、`file` 或受控
`remote`。`file` source 会独立渲染目标文件资源；当前 processor 的
`params.args` 只进入随后执行的脚本 `input.args`。完整输入输出、timeout 和可用 API 以
[Scripting API](scripting-api.md) 为准；不要把它当作 Node.js 或通用系统访问。

## file stage

当前内建 type 为 `inject_nodes`、`merge`、`yaml_patch`、`json_patch`、
`template`、`script`。

### merge

普通 `FileSpec` 流程应使用 `content`：processor 把当前完整
`FileDocument.Content` 当作 `current`，把 `params.content` 当作后续
`overlay`，依 `mode` 计算新正文。`content` 不能为空；`append` 可用
`separator` 指定连接符。

支持的 mode：

| mode | 运算 |
| --- | --- |
| `append` | 按顺序连接，`separator` 缺省为换行。 |
| `replace` | 取最后一个选中 part。 |
| `yaml_overlay` | 递归合并 YAML object；后值覆盖 scalar/array，object 内 `null` 删除 key。 |
| `json_overlay` | 与 YAML overlay 相同，输出稳定缩进 JSON。 |
| `yaml_override` | 有序 YAML object override，支持数组和强制替换运算符。 |
| `json_override` | 同一套有序运算，输入必须是有效 JSON，输出为有效缩进 JSON。 |
| `ini_override` | 按顺序执行保留格式的 INI section override。 |

`yaml_override` 与 `json_override` 在每个 object 中按出现顺序解释 key：

| 写法 | 含义 |
| --- | --- |
| `key` | object 对 object 递归合并；其他类型整体替换。 |
| `+key` | 把数组前插到现有数组。 |
| `key+` | 把数组追加到现有数组。 |
| `key!` | 不递归，强制整体替换。 |
| `<key>` | 把看起来像运算符的 key 当作字面名称。 |
| `key: null` | 删除 key。 |

数组目标不存在时，前插/追加会创建它；目标或 override 值不是数组时返回
`file_merge_failed`，并尽可能给出 `part` 与 JSON Pointer `path`。

`ini_override` 的 section 名匹配忽略大小写，patch section 按源码顺序运行：

- `[Section]`：合并；同名 assignment key 被替换，其他不重复记录追加；
- `[+Section]`、`[Section+]`：分别前插、追加整个 section body；
- `[Section!]`：替换整个 body；
- `[Section-]`：删除所有同名 section，body 只能含空行或注释；
- `[<Section+>]`：转义成字面 section 名，不解释尾部运算符。

INI assignment key 的比较区分大小写；普通记录按去空白后的整行去重。输出继承
base 的 BOM 和换行风格。

### yaml_patch / json_patch

两者都要求非空 `ops`，路径使用 JSON Pointer（单段可省略开头 `/`）。支持
`add`、`replace`、`remove`、`test`、`move`、`copy`；`move`/`copy` 使用
`from`，需要值的操作使用 `value`。`yaml_patch` 解析并重新编码 YAML，
`json_patch` 输出稳定的两空格缩进 JSON。

### template

把正文中的占位符替换为字符串：

- `open`、`close` 缺省为 `{{`、`}}`；
- `vars` 与请求 `meta` 合并，同名时 `vars` 优先；
- 未定义变量或未闭合 delimiter 会让整步失败。

### inject_nodes

runtime registry 仍会列出 `inject_nodes`，但它要求直接调用内部 registry
时提供 `FileParts`；当前 CLI、HTTP 和 `pkg/sandrone` 的 `FileSpec` 流程都不
产生这些 parts，因此不能把它用于公开文件处理链。typed 文件由 driver 直接
编译节点。

### file script

file-stage `script` 可修改完整文件 envelope，并可使用受控资源 API。参数、
返回转换和安全边界只在 [Scripting API](scripting-api.md) 详述。

### Web 内置字符串替换预设

Web 中所有受管社区配置预设的默认状态、精确输出、风险、依赖、冲突与版本边界
统一见[社区配置预设](community-config-presets.md)。它们只复制普通 `merge` 或
`script` processor，不改变本页定义的 processor type 和执行顺序。

Web 文件处理器编辑器为 Mihomo、sing-box 和 Shadowrocket 提供
“GitHub 规则源镜像替换”快捷项。它不是新的 processor type；选择后会在链末尾
追加一个普通的 file-stage `script`。脚本源码是通用的有序字面量字符串替换器，
具体映射由 processor 参数提供：

| `params.args` | 契约 |
| --- | --- |
| `preset_id` | 固定为 `github-rule-source-mirror`，用于识别内置预设。 |
| `replacements` | 有序的 `[source, destination]` 字符串二元组数组。 |

脚本按数组顺序执行，每一组都替换正文中的全部字面匹配，不使用正则表达式。
参数缺失、不是数组或数组元素不是两个字符串时，当前 processor 失败。

预填脚本把下列已知规则库前缀改写为 jsDelivr：

| GitHub Raw | 默认目标 |
| --- | --- |
| `https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/` | `https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@meta/` |
| `https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/` | `https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/` |
| `https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/` | `https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/` |

这些只是可编辑的默认替换值。用户可以在普通脚本参数编辑器中修改
`replacements`，换成其他镜像或追加映射；通用脚本源码不绑定 GitHub 或
jsDelivr。没有匹配项时正文保持不变，不产生 warning。删除该 script 即停止
输出时改写，结构化规则集字段本身不会被迁移。

旧版带 `sandrone:file-preset=github-rule-source-rewrite` marker 的内联脚本仍会被
识别为同一快捷项，以抑制重复添加；Web 不会改写其脚本正文或已保存名称。

## 失败与原子性

链在第一个 build 或 apply 错误处停止，后续 processor 不运行：

- 参数在 apply 前严格解码；拼写错误不会被静默忽略；
- 每一步只有成功返回后才替换链的 current 值；
- `merge`、patch 和 script 都先在临时表示上计算；失败的步骤返回进入该步骤前
  的文件，不发布半成品；
- service 的 parse、render 或 file 入口在链失败时返回 error，不返回可消费的
  部分结果，也不会改写已保存的 `FileSpec` 或 source。
