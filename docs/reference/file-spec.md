# FileSpec 参考

`FileSpec` 描述一个可保存、可渲染的文件资源。本页是其字段与缺省语义的
现行契约；处理链参数见 [Processors 参考](processors.md)，HTTP 包装见
[HTTP API 参考](http-api/README.md)。

## 顶层字段

| 字段 | 类型 | 契约 |
| --- | --- | --- |
| `name` | string | 资源名。保存资源时必须非空；内联渲染时可由请求的 `name` 补入。 |
| `display_name` | string | 可选显示名；保存时会去除首尾空白。 |
| `kind` | string | 必填且区分大小写，必须是下列 canonical kind 之一。 |
| `source` | object | `FileSource`。typed 文件可以用空对象选择内建 base；`static` 必须给出可读取的 source。 |
| `config` | object | 仅供 typed 文件使用；`static` 出现此字段即报错。 |
| `processors` | array | `ProcessorSpec` 列表，按声明顺序选择并执行对应 stage。 |
| `meta` | object<string,string> | 随文件保留的调用方元数据。 |
| `created_at`、`updated_at` | RFC 3339 time | 可选资源时间戳，由存储层原样往返。 |

`kind` 不做大小写转换、去空白或旧名称迁移。有效值只有：

| canonical kind | 文件语法 | 缺省 base | 节点渲染目标 |
| --- | --- | --- | --- |
| `static` | 不解释 | 无 | 无 |
| `mihomo` | YAML | 内建 Mihomo 基础配置 | Mihomo proxy 列表 |
| `sing-box` | JSON | 内建 sing-box 基础配置 | sing-box outbound/endpoints |
| `shadowrocket` | INI | `[General]` | 无；配置与客户端订阅节点分离 |

因此 `kind: static` 也必须显式写出；空字符串、`Static`、` static ` 和未注册值
均为无效输入。

## FileSource

`source.type` 在读取时去除首尾空白并按小写匹配：

| `type` | 字段 | 读取语义 |
| --- | --- | --- |
| `inline` | `content` | 直接以 UTF-8 字符串字节作为 base。空字符串也是有效内容。 |
| `remote` | `remote` | 受控抓取 `remote.url`；URL 必须为 `http` 或 `https`。 |

`remote` 对象支持：

- `url`：必填；
- `user_agent`、`proxy`、`timeout_ms`：可选抓取参数；
- `cache_ttl_seconds`：已保存 File 的执行作用域中，大于零时允许使用持久化抓取
  缓存；inline FileSpec 不读写持久缓存。未启用 store 时也不能形成持久缓存。
  零值继承项目设置的 `cache_defaults.remote_fetch_ttl_seconds`；
  两者都为零时才会每次重新抓取。

保存文件时，完整 `FileSpec` 作为单个 JSON record 写入 Store。`inline`
正文继续保存在 `source.content` 中；`remote` 只保存远程描述，不预先抓取。
读取 `mode=spec` 或 MCP definition resource 会返回这一完整定义。

对 typed kind，`source.type` 为空表示使用该 driver 的内建 base。若给出
`inline` 或 `remote`，读取结果就是自定义 base，并必须能按该 kind
的 YAML、JSON 或 INI 语法解析。对 `static`，空 `source.type` 会在读取时失败。

## FileConfig

typed 文件的公共 `config` **只有**两个字段：

| 字段 | 类型 | 语义 |
| --- | --- | --- |
| `subscriptions` | string[] | 仅适用于声明了节点 renderer 的 typed kind；按数组顺序解析订阅并拼接节点。Shadowrocket 不允许非空值。 |
| `settings` | JSON object | typed kind 必填；原样交给当前 kind 的 driver 严格解码。 |

`config` 的其他字段会被拒绝。组、规则集和规则必须放在
`config.settings`。typed kind 的 `config`、`settings` 以及其中的 `groups`、
`rule_sets`、`rules` 都不能省略；三个字段必须是数组，但允许显式为空。
`settings` 不能是数组、标量或 `null`；未知顶层字段以及已知字段上的 `null`
均报 `invalid_argument`。

在 MCP tool 的 JSON wire 上，`config.settings` 直接是 object；调用方不应把
它编码成 JSON 字符串。各 canonical kind 的可发现 schema、source rules 与
examples 见 [MCP resources](mcp.md#resources-与-schema-templates)。

对 Mihomo 和 sing-box，`subscriptions` 省略和显式 `[]` 都产生零个节点；重复的
订阅名不会去重，解析任一订阅失败会终止整个文件生成。Shadowrocket FileSpec
不解析订阅，非空 `subscriptions` 返回 `invalid_argument`；其节点订阅通过
`shadowrocket-proxies` 独立交付。

## typed settings 完整性

三个 driver 都只接受调用方已经完整物化的 settings：

| kind | `settings` 允许的顶层字段 | 必填字段 | 显式 `[]` |
| --- | --- | --- | --- |
| `mihomo` | `adaptive_groups`、`groups`、`rule_sets`、`rules` | `groups`、`rule_sets`、`rules` | 对应的 `proxy-groups`、`rule-providers` 或 `rules` 为空 |
| `sing-box` | `groups`、`rule_sets`、`rules` | `groups`、`rule_sets`、`rules` | 对应的 selector、`route.rule_set` 或 `route.rules` 为空 |
| `shadowrocket` | `adaptive_groups`、`groups`、`rule_sets`、`rules` | `groups`、`rule_sets`、`rules` | 对应的 `[Proxy Group]`、规则集映射或 `[Rule]` 不生成条目 |

省略整个 `config`、省略 `settings`、提交 `settings: {}` 或漏掉任一必填字段都返回
`invalid_argument`。空数组表示调用方明确选择该输出集合为空，不触发任何后端默认值。

Mihomo 与 sing-box 的 `groups`、`rule_sets`、`rules` 使用客户端结构：

- Mihomo `groups` 和 `rule_sets` 是 object 数组，`rules` 是 string 数组；
- sing-box 三者都是 object 数组。

Shadowrocket 的 settings 还执行字段级严格校验：

- `groups[]` 必须有 `name`、`type`，且必须二选一给出 `proxies` 或
  `policy-regex-filter`；使用 `PROXY` 引用客户端当前代理，或用
  `policy-regex-filter` 匹配客户端订阅节点；
- `rule_sets[]` 使用 `name`、`type`、`url`；
- `rules[]` 是 Shadowrocket 规则字符串；
- Web/HTTP 兼容元数据 `adaptive_groups` 只接受已声明的 type 和 region 值。

所有 settings 的未知字段都会失败。Shadowrocket 的嵌套对象也拒绝未知字段；
Mihomo 和 sing-box 的客户端 object 内容保持开放，以容纳各自客户端字段。
当前三个 compiler 的组输出都由 `groups` 决定；`adaptive_groups` 只作为已知
settings 结构被解码（Shadowrocket 还会校验其取值），不会替代 `groups` 或
自动合成额外组。

## 编译与所有权

typed 文件的完整生成顺序只在[文件管线](../architecture/file-pipeline.md)说明。
本页只定义 settings 输入与 driver 拥有的输出位置。

driver 会重建其拥有的节点和策略位置，而不是简单把节点附加到 base：

- Mihomo 重建 `proxies`、`proxy-groups`、`rule-providers`、`rules`；
- sing-box 重建 `outbounds`、`route.rule_set`、`route.rules`，缺少
  `route.final` 时补为 `Proxy`；渲染到 endpoint 的节点写入 `endpoints`；
- Shadowrocket 清空 `[Proxy]`，重建 `[Proxy Group]`、`[Rule]`，保留其他 section；
  它不会把 Sandrone 订阅节点写进 `.conf`。

要在 typed 编译后做结构化修改，使用 file-stage `merge`；开放式逻辑使用
file-stage `script`。二者都在编译完成后按 `processors` 的声明顺序执行。

## 最小 typed 示例

### Mihomo

```yaml
name: mihomo.yaml
kind: mihomo
source: {}
config:
  subscriptions: [provider]
  settings:
    groups:
      - name: Proxy
        type: select
        proxies: [$nodes, DIRECT]
    rule_sets: []
    rules: [MATCH,Proxy]
```

### sing-box

```yaml
name: sing-box.json
kind: sing-box
source: {}
config:
  subscriptions: [provider]
  settings:
    groups:
      - type: selector
        tag: Proxy
        outbounds: [$nodes, direct]
    rule_sets: []
    rules:
      - outbound: Proxy
```

### Shadowrocket

```yaml
name: shadowrocket.conf
kind: shadowrocket
source: {}
config:
  settings:
    groups:
      - name: Proxy
        type: select
        proxies: [PROXY, DIRECT]
    rule_sets: []
    rules: [FINAL,Proxy]
```

这三个示例都使用内建 base。Shadowrocket 的节点订阅须另行添加
`shadowrocket-proxies` 分享链接。若要自带 base，把 `source` 改成完整的
`inline` 或 `remote` source。
