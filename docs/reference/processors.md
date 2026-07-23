# Processors 参考

Processor 是显式声明、按顺序执行的转换步骤。本页定义当前 registry、参数和
失败语义。脚本的 envelope、注入 API 与 sandbox 见
[Scripting API](scripting-api.md)；错误对象字段见 [错误参考](errors.md)。

## ProcessorSpec

```yaml
type: rename
stage: nodes
name: optional-label
params:
  mode: prefix
  value: "HK-"
```

| 字段 | 契约 |
| --- | --- |
| `type` | 必填，必须是已注册 processor type。 |
| `stage` | `nodes` 或 `file`。仅当该 type 只注册在一个 stage 时可省略并推断。 |
| `name` | 可选声明名；不改变 type 或执行语义。 |
| `params` | object；每个内建 processor 都拒绝未知字段。 |

`script` 同时注册在 `nodes` 和 `file`，所以使用时必须显式写 `stage`。未知
type 返回 `processor_unknown`；stage 无法唯一推断或参数无效返回
`processor_config_invalid`。

在 MCP tool 的 JSON wire 上，`params` 直接是 object；调用方不应把它编码成
JSON 字符串。可发现的 processor 摘要与逐项 schema URI 见
[MCP resources](mcp.md#resources-与-schema-templates)，本页继续定义领域语义。

每次运行只选择当前 stage 的项，并保留它们在原数组中的相对顺序。前一步输出
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

保留每个 key 首次出现的节点：

- `strategy: identity`（缺省）：使用 `type`、`server`、`port`、`uuid`、
  `password`；
- `strategy: name`：只使用节点名；
- `strategy: fields`：按 `fields` 声明顺序组合 key，数组不能为空。

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

`layer`、`method`、`core`、`url`、`ntp_server`、`expected_status`、
`timeout_ms`、`attempts`、`concurrency`、`cache_ttl_seconds`。
省略值由 probe service 的规范化和运行时默认值处理。

处理结果由以下参数决定：

- `fail_mode: keep`（缺省）：保留失败节点；
- `fail_mode: drop`：丢弃失败节点；
- `fail_mode: error`：遇到第一个失败结果即让整步失败；
- `annotate: true`：重写该节点所有 `probe.*` meta，写入 layer、method、
  alive、duration、checked_at、error_code 等已有结果；
- `sort: duration`：存活节点在前并按延迟升序；失败节点在后，平局保持输入顺序。

runner 返回的结果数必须与输入节点数相同。probe report warning 会并入 processor
warning；runner 错误直接终止链。

### nodes script

`script` 的 `engine` 当前只能是 `js`，source 可为 `inline`、`file` 或受控
`remote`。完整输入输出、timeout 和可用 API 以
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

## 失败与原子性

链在第一个 build 或 apply 错误处停止，后续 processor 不运行：

- 参数在 apply 前严格解码；拼写错误不会被静默忽略；
- 每一步只有成功返回后才替换链的 current 值；
- `merge`、patch 和 script 都先在临时表示上计算；失败的步骤返回进入该步骤前
  的文件，不发布半成品；
- service 的 parse、render 或 file 入口在链失败时返回 error，不返回可消费的
  部分结果，也不会改写已保存的 `FileSpec` 或 source。
