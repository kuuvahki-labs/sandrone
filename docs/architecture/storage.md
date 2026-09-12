# 存储架构

## 目标与范围

Sandrone 不要求数据库。持久化层保存命名资源和统一项目设置，并为内部缓存提供统一 key 空间；转换、文件生成和探测仍是 service 的请求级编排。

默认使用本地文件系统，也支持 S3-compatible 对象存储和嵌入方自定义 Store。
Store 不要求数据库事务、跨进程锁或自动 schema 迁移；复合操作的一致性由下文说明。

## `Store`

`internal/store.Store` 是 service 使用的最小持久化接口：

| 操作 | 契约 |
| --- | --- |
| `Read` | 读取一个 key 的完整 bytes |
| `Write` | 覆盖写入一个 key |
| `Delete` | 删除一个 key |
| `List` | 列举前缀下的 entry |
| `Stat` | 读取单个 entry metadata |

公开嵌入 API 暴露同构接口，因此自定义后端不需要 import `internal/store`。

可选优化接口不扩大公开 Store 的必选方法；自定义后端未声明并发读取能力时按串行读取。
接口与 key 校验见 [`internal/store/store.go`](../../internal/store/store.go)，公开适配点见
[`pkg/sandrone/sandrone.go`](../../pkg/sandrone/sandrone.go)。

## Key 安全

所有 key 都是相对于 Store root 的 slash path。实现必须拒绝：

- 空 key 和 NUL。
- 绝对路径或 Windows drive prefix。
- 反斜杠。
- 空 path segment。
- `.`、`..` 和目录穿越。

service 对资源名和备份条目重复应用同一 `CleanKey` 语义。Store key 不是任意
宿主文件路径，entrypoint 也不能把用户输入直接拼成 OS path。

自定义 Store 的 `List` 必须返回安全、规范且不重复的 keys；备份和资源枚举会把这一点当作后端契约。

## `FSStore`

`FSStore` 基于 `afero.Fs` 实现 Store。运行服务通常把它放在 `afero.BasePathFs` 后，使所有 key 都限制在配置的数据目录内；测试可以替换为内存或只读 afero 文件系统。

`FSStore` 用进程内读写锁保护单次方法调用。

普通 `Write` 是覆盖写，不承诺临时文件 rename、fsync 或崩溃恢复。底层文件系统的 durability 和多个 Store 实例之间的协调属于部署边界。

## `S3Store`

`S3Store` 使用 AWS SDK for Go v2 直接实现同一 Store 契约，不把对象存储模拟成
`afero.Fs`。运行时通过 `SANDRONE_STORAGE_BACKEND=s3` 选择它；filesystem
仍是默认后端。

每个逻辑 key 映射为 namespace 下的一个对象，普通写入和 `WriteAtomic` 都使用
单对象 PUT。删除缺失 key 返回 not-found；列举会合成中间目录并校验 keys。
后端须对读取、覆盖、删除和列举提供强一致语义，才能满足资源管理和恢复的预期。
实现见 [`s3.go`](../../internal/store/s3.go)。

S3 endpoint、region、bucket、namespace 和显式 access key 由进程环境提供。
Sandrone 不调用 AWS 默认凭据链，不读取 shared profile 或 instance metadata。
凭据不进入 Store entry、项目设置或备份。

## `MetaStore`

`MetaStore` 构建在任意 Store 上，把 JSON 资源映射到 keys。它不是另一个
持久化后端，也不向 service 暴露数据库式查询。

它负责编解码 subscription、file、share 并列举资源摘要。列表每次列举当前前缀，
以发现外部变更；已读取的摘要可以复用。S3 按 opaque ETag 判断版本并定期强制复核，
GET 权限变化可能延迟到复核时发现。文件系统及自定义 Store 按正文指纹复用解析结果。
本实例写入、删除和恢复后失效，返回独立值；读取和解码错误不缓存。
复核期限与缓存实现见 [`meta_cache.go`](../../internal/store/meta_cache.go)。

每个 file 使用一个 `files/<name>.json` record 保存完整 `FileSpec`。inline
正文留在 `source.content`，不会拆成相邻 raw key，也不会在保存时改写 source
类型。覆盖和删除 file 因此都是单个资源 key 操作。

项目设置由独立的 `SettingsStore` 严格解码，并通过 `AtomicWriter` 在 Store 根部
写入 `settings.json`。FSStore 使用临时文件 rename；S3Store 使用单对象 PUT，
由对象存储的完整对象发布语义保证读者不会得到部分正文。`Report`、`FileResult`、`ProbeResult` 和编译后的
客户端文件不是 MetaStore 管理资源。内部 cache 可以暂存请求结果，但它有独立
前缀和 TTL 语义。

## `Cache`

`internal/cache.Cache` 是 service 使用的非权威缓存边界。它只处理 opaque key、opaque
value 和整个 key 的 TTL，不解释 key 路径或 value 内容：

| 操作 | 契约 |
| --- | --- |
| `Get` | 按单个 key 返回未过期 value 及其绝对过期时间；缺失和过期返回 miss |
| `Set` | 按单个 key 写入 opaque value 和一个 TTL |
| `Delete` | 删除单个 key |
| `Clear` | 清空该 Cache 实例管理的全部 key |

service 分别持有权威 `Store` 与非权威 `Cache`；自定义 Cache 无需实现资源 Store。
业务层拥有 value 的结构与匹配规则，Cache 只保存 opaque bytes，当前实现使用
Store-backed Cache。

整个 key 共用一个绝对过期时间。普通读取或保存可按较短的新 TTL 缩短期限，但不会
延长旧 value；`refresh` 跳过旧值并重建 TTL。同一 refresh 请求后续访问会复用本次
新值。共享缓存的并发更新可能降低命中率，不改变权威资源。

持久缓存共有三类 key 前缀：

| key 前缀 | 缓存值 | TTL 来源 |
| --- | --- | --- |
| `remote_fetch` | 受控 HTTP(S) 响应 | `RemoteInput.cache_ttl_seconds`，零值继承项目默认 |
| `probe` | 按连接保存的节点观测 | probe 请求，零值继承项目 `cache_defaults.probe_ttl_seconds` |
| `subscription_snapshot` | 已保存订阅处理前后的 canonical `NodeSet` 与依赖 revision | Subscription 三态覆盖或项目默认 |

Subscription 的 `snapshot_ttl_seconds` 是 nullable 三态字段：省略时
继承项目默认，显式 `0` 关闭，正数覆盖；项目默认值为 `0`。持久缓存只属于已保存的 Subscription
或 File：inline FileSpec、直接 parse/render/convert、临时 diagnose 和未保存草稿
不读写任何持久层，只保留请求内 memo。share 没有独立缓存层，但生成已保存订阅目标时
可以复用订阅执行快照。
过大的 subscription-snapshot 会跳过缓存写入，仍返回本次执行结果。

订阅执行快照的 identity 包含构建身份、Subscription 定义、请求上下文和
remote/probe/script 执行设置，不包含输出 target。它保存处理前后的 canonical
`NodeSet`，供 preview、renderer、typed file、share 和脚本订阅调用复用。
命中前核对实际依赖资源的 revision，定义变化立即 miss；远程内容与 probe 观测则
可以在 snapshot TTL 内保持旧值。`refresh` 跳过各持久缓存读取，成功后按当前 TTL
重新填充。最终目标正文每次生成，不作为持久缓存结果保存。

probe 和 scheduler 的运行时可用性不参与快照 identity，因此具备 probe 能力的
实例可刷新共享快照，其他实例直接消费。双方仍需上述 identity 一致且 snapshot TTL
为正数。无 probe 能力的实例冷 miss 时，processor 会 warning 并继续；这种跳过 probe
的降级结果不回填快照，避免覆盖可复用的测活结果。

请求内 memo 复用相同执行变体及资源读取，返回独立值，同作用域写入后失效。
它不跨请求或 service，不代表 Store 事务快照；同时发生的冷 miss 可以各自执行。

手动清理调用 `Cache.Clear`。当前 Store-backed 清理可能部分成功，并发请求也可能
重新填充；它不修改权威资源或 TTL 设置。接口见[缓存管理 API](../reference/http-api/settings.md#缓存管理)。

### 定时更新

长驻 HTTP、MCP HTTP 和合并 serve 模式启动进程内更新器，直接 CLI 与嵌入
`Engine` 不启动。运行时禁用 scheduler 时，保存的计划仍保留但不执行。

每次触发按配置顺序运行目标，不带请求 args，使用 `refresh=true`：subscription
执行 preview，file 执行完整 render。这些操作预热适用缓存，不覆盖 TTL，也不保存
最终正文或历史 report。

同一进程中，重叠触发被跳过，单个目标失败后继续后续目标。启动和计划热更新本身
等待下一次 cron；显式运行一次复用当前 targets 和重叠保护，不移动下次计划。
状态仅在内存中，关闭会取消当前任务并等待返回。多个实例没有 leader 选举或分布式锁，
共享 Store 的部署需自行选择启用调度的实例。

cron、状态和一次性运行接口见[项目设置接口](../reference/http-api/settings.md#定时更新)。

## Key 布局

当前 service 使用的主要 keys 是：

```text
subscriptions/<name>.json
files/<name>.json
settings.json
shares/<id>.json
cache/<cache-key-prefix>/subscriptions/<name>.json
cache/<cache-key-prefix>/files/<name>.json
```

其中：

- `subscriptions/`、`files/` 和 `shares/` 是领域资源；根部
  `settings.json` 是统一项目设置。
- `cache/` 只用于可重建的内部加速，不是权威资源。
- 每种适用缓存与保存资源组合最多一个 key；Store-backed Cache 将它保存成一个
  文件。同一资源的不同请求变体或节点观测由对应业务 value 自行组织，全部共享该
  key 的单一绝对过期时间。资源名中的安全 `/` 与领域资源一样形成子目录。
- 同一 URL 或连接出现在不同资源时分别缓存，不跨 Subscription/File 复用。
- 未知安全 key 可以由自定义集成保存；raw Store 备份会保留非 cache key。

文件正文、metadata 与运行时生成结果的关系见[文件管线](file-pipeline.md)。

## `Coordinator`

`Coordinator` 在 Store 之上增加两个复合操作：

- `View`：共享锁内执行一致读取回调。
- `Update`：独占锁内执行复合修改回调。

回调接收底层 raw Store，避免持锁期间重复获取同一把锁。service 中的资源与备份操作
共享 Coordinator；已有 Coordinator 会直接复用。

Coordinator 提供的是单进程 isolation：

- export 可以在同一个 read view 中完成 List 与多次 Read。
- restore 可以阻止同一 service 的普通读写观察到替换中间态。
- MetaStore 的资源操作和备份操作共享同一 coordinator 边界。

它不是事务管理器。`Update` 回调中前一个 Write 成功、后一个 Write 失败时，不会自动回滚；它也不提供 write-ahead log、crash atomicity 或多个 Sandrone 进程之间的锁。

多个进程或 Vercel Function instance 共享同一 S3 namespace 时，Coordinator
不能阻止跨实例交错。backup restore 必须在停止其他 writer 的维护窗口执行；
Sandrone 不为对象存储增加分布式锁。

实现见 [`internal/store/coordinated.go`](../../internal/store/coordinated.go)。

## 普通写入与一致性

一致性按操作类型区分：

- 单 key 资源更新使用覆盖写。
- share 使用相同的覆盖写语义；显式 ID 已存在时由新记录替换。
- 删除 file 不级联删除它引用的 subscription、其它 file 或 share。
- cache miss、损坏或写入失败不能改变权威资源。

因此“没有交错观察”不等于“发生错误后必然回到旧状态”。需要强原子发布的自定义后端可以在自己的 Store/Coordinator 实现中提供更强保证。

## 备份与恢复边界

Store 备份面向管理员搬运原始存储，不重新编码 `Subscription` 或 `FileSpec`。
导出在 Coordinator read view 中读取所有非目录、非 cache keys，并保留未知安全
key。

cache 被排除，因为它可重建、可能过期，也不应决定恢复后的权威状态。

恢复遵守以下顺序：

1. 在修改 Store 前完整解码并校验归档、schema、key tree，以及可选
   `settings.json` 的严格设置契约。
2. 进入 Coordinator 独占 update。
3. 快照旧的非 cache bytes。
4. 删除现有文件，包括 cache，再写入备份内容；`settings.json` 使用原子写入，
   文件系统后端的权限为 `0600`。
5. 普通写入失败时，尝试恢复旧的非 cache bytes；cache 保持为空。
6. 替换成功后重新载入动态项目设置；若载入意外失败，则回滚 Store 和内存设置。

这是 best-effort rollback，不是 crash-atomic restore：

- 进程在删除与写入之间崩溃时可能留下部分 Store。
- rollback 自身也可能失败，并会作为复合错误报告。
- 共享同一后端的其它进程不受当前 Coordinator 约束。
- 只接受当前支持的 storage schema。唯一的读取期清理是重写
  `settings.json` 中已经移除的启动字段，避免旧的敏感值继续保留或进入后续备份；
  其他结构不在恢复时自动迁移。

`storage_schema_version=1` 是备份容器和 key tree 的版本，不是每个 JSON
资源 shape 的迁移承诺。恢复会原样写回资源 bytes；当前运行时要求 file record
本身是包含 inline 正文或 remote 描述的完整 `FileSpec`，不会从其它 raw key
补齐或迁移定义。

备份包含 Store 原始 bytes，可能包括订阅 URL、节点凭据、脚本和项目设置。
归档不提供加密或签名保证，必须由部署方保护传输、访问和静态存储，并在恢复前
确认来源可信。

归档 wire、大小限制、HTTP 鉴权和错误响应属于管理接口契约，见
[项目设置与备份接口](../reference/http-api/settings.md)；本页只定义存储一致性
与恢复后果。
