# 存储架构

## 目标与范围

Sandrone 不要求数据库。持久化层保存命名资源和统一项目设置，并为内部缓存提供统一 key 空间；转换、文件生成和探测仍是 service 的请求级编排。

稳定目标是：

- 本地服务默认使用文件系统目录，也可以显式选择 S3-compatible 对象存储。
- 测试可以使用内存或只读文件系统。
- 嵌入方可以提供自定义 Store。
- 复合读取和维护操作在单进程内有一致性边界。
- 备份能够搬运完整的非 cache Store，而不解释领域对象。

Store 不提供数据库事务、跨进程锁或自动 schema 迁移。

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

接口与 key 校验定义见 [`internal/store/store.go`](../../internal/store/store.go)，公开适配点见 [`pkg/sandrone/sandrone.go`](../../pkg/sandrone/sandrone.go)。

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

每个逻辑 key 映射为配置 namespace 下的一个对象。实现只依赖
`GetObject`、`PutObject`、`DeleteObject`、`HeadObject` 和分页
`ListObjectsV2`：

- 普通写入和 `WriteAtomic` 都是单对象 PUT；后者不模拟 rename，也不使用临时
  对象。
- 删除先检查对象存在性，使删除缺失 key 继续返回 `os.ErrNotExist`。
- `List` 读取全部分页，移除物理 namespace，拒绝非法或重复逻辑 key，并合成
  中间目录 entry。
- `Stat` 优先读取对象 metadata；只有后代对象存在时才返回合成目录。
- provider 的 `NoSuchKey`/`NotFound` 映射为现有 not-found 语义。

支持的 S3-compatible 服务必须对对象读取、覆盖、删除和列举提供强一致语义。
Cloudflare R2 是文档化并执行集成验证的首个目标。实现不依赖 ACL、versioning、
object lock、multipart upload、presigned URL 或 provider 专属 metadata。

S3 endpoint、region、bucket、namespace 和显式 access key 由进程环境提供。
Sandrone 不调用 AWS 默认凭据链，不读取 shared profile 或 instance metadata。
凭据不进入 Store entry、项目设置或备份。

## `MetaStore`

`MetaStore` 构建在任意 Store 上，把 JSON 资源映射到 keys。它不是另一个
持久化后端，也不向 service 暴露数据库式查询。

它负责：

- subscription、file 和 share 的 JSON 编解码。
- 资源摘要列举。
- share 的覆盖保存。

每个 file 使用一个 `files/<name>.json` record 保存完整 `FileSpec`。inline
正文留在 `source.content`，不会拆成相邻 raw key，也不会在保存时改写 source
类型。覆盖和删除 file 因此都是单个资源 key 操作。

项目设置由独立的 `SettingsStore` 严格解码，并通过 `AtomicWriter` 在 Store 根部
写入 `settings.json`。FSStore 使用临时文件 rename；S3Store 使用单对象 PUT，
由对象存储的完整对象发布语义保证读者不会得到部分正文。`Report`、`FileResult`、`ProbeResult` 和编译后的
客户端文件不是 MetaStore 管理资源。内部 cache 可以暂存请求结果，但它有独立
前缀和 TTL 语义。

## `Cache`

`internal/cache.Cache` 是 service 使用的非权威缓存边界，只提供三个能力：

| 操作 | 契约 |
| --- | --- |
| `GetJSON` | 读取仍在有效期内的 JSON 值；缺失、过期、损坏或后端失败都按 miss 处理 |
| `PutJSON` | 写入值并在写入时确定到期时间；TTL 非正时不写入 |
| `DeleteLayer` | 清空一个由 service 选择的 canonical 层，用于资源变更后的广泛失效 |

service 分别持有权威 `Store` 与非权威 `Cache`，因此自定义 Cache 不需要充当
资源 Store。当前仓库只内建 Store-backed 实现：它把 envelope 写到
`cache/<layer>/`，使用无缩进和尾随换行的紧凑 JSON；读取过期 entry 时
best-effort 删除。没有运行时 backend
registry、第三方 cache 依赖或缓存管理 HTTP API；以后更换为内存或远程实现时，
应保持相同的 miss/failure 语义，而不是让 cache 可用性影响业务结果。

持久 TTL 层共有五个：

| canonical 层 | 缓存值 | TTL 来源 |
| --- | --- | --- |
| `remote_fetch` | 受控 HTTP(S) 响应 | `RemoteInput.cache_ttl_seconds`，零值继承项目默认 |
| `probe` | 完整批次 `ProbeResult` | probe 请求，零值继承项目 `cache_defaults.probe_ttl_seconds` |
| `subscription_traffic` | 远程订阅用量 | 项目设置默认 |
| `subscription_render` | 已保存订阅的完整 `RenderResult` | Subscription 三态覆盖或项目默认 |
| `file_render` | 已保存文件的完整 `FileResult` | FileSpec 三态覆盖或项目默认 |

Subscription/FileSpec 的 `render_cache_ttl_seconds` 是 nullable 三态字段：省略时
继承对应项目默认，显式 `0` 关闭，正数覆盖。两个结果缓存的项目默认值
均为 `0`，所以升级不会自动缓存生成结果。inline FileSpec、直接 parse/render/
convert、preview 不使用结果缓存；share 没有独立缓存层，但生成已保存目标时可以
复用目标自身的结果缓存。超过 16 MiB 的最终正文不会写入结果缓存。

结果 key 包含 cache schema、构建版本/revision、完整资源定义、目标格式和影响
执行的请求参数。订阅、文件或项目设置变更后，service 会清空可能受
影响的结果层；订阅变更还会清空 traffic 层。`refresh` 请求跳过结果、
remote-fetch 和 probe 的缓存读取，成功执行后仍按当前 TTL 重新填充。
`ValidateFile` 不读取 file-result cache。除此之外，订阅解析和文件递归各有一次
请求内 memo，用于去重同一调用中的重复依赖；它们不持久化、没有 TTL，也不是
可配置 cache 层。

### 定时更新

长驻的 HTTP、MCP HTTP 和合并 serve 模式会启动一个进程内定时更新器；直接 CLI
操作和嵌入 `Engine` 不启动它。项目设置用一个 cron 计划和一组显式目标控制该
更新器。每次触发按配置顺序逐个执行目标，不并发物化目标：

- subscription 目标执行不带 args、`refresh=true` 的 preview，完成订阅物化和
  nodes-stage processors，但不要求或生成某个 renderer 目标；
- file 目标执行不带 args、`refresh=true` 的完整文件 render，最终格式由
  `FileSpec.kind` 决定。

两类操作都跳过本次 remote-fetch、probe 和适用的最终结果缓存读取，并在成功时
按资源与项目现有 TTL 填充缓存。调度器不覆盖 TTL，不保存 preview、report、
历史记录或额外产物。subscription preview 没有 subscription-render 结果缓存；
它预热的是物化过程中实际使用的 remote-fetch 与 probe 层。file render 还可
预热 no-args 请求对应的 file-render 层；带 args 的请求具有不同 cache key，
不会命中该最终结果。

一个进程只运行一个调度任务；上次触发尚未结束时，新触发会被计数并跳过，不会
排队。单个目标失败只记录错误并继续后续目标，不立即重试。启动和计划热更新都
等待下一次 cron 时间，不立即补跑；关闭时取消 service context，并等待当前任务
返回。状态只保存在内存中，重启后清零。多个 Sandrone 实例之间没有 leader 选举
或分布式锁，因此共享 Store 的部署必须自行确保只有预期实例启用调度。

设置 wire、cron 约束和状态接口见[项目设置接口](../reference/http-api/settings.md#定时更新)。

## Key 布局

当前 service 使用的主要 keys 是：

```text
subscriptions/<name>.json
files/<name>.json
settings.json
shares/<id>.json
cache/probe/<hash>.json
cache/remote_fetch/<hash>.json
cache/subscription_traffic/<hash>.json
cache/subscription_render/<hash>.json
cache/file_render/<hash>.json
```

其中：

- `subscriptions/`、`files/` 和 `shares/` 是领域资源；根部
  `settings.json` 是统一项目设置。
- `cache/` 只用于可重建的内部加速，不是权威资源。
- 未知安全 key 可以由自定义集成保存；raw Store 备份会保留非 cache key。

文件正文、metadata 与运行时生成结果的关系见[文件管线](file-pipeline.md)。

## `Coordinator`

`Coordinator` 在 Store 之上增加两个复合操作：

- `View`：共享锁内执行一致读取回调。
- `Update`：独占锁内执行复合修改回调。

回调接收底层 raw Store，避免在持锁期间再次通过 coordinator 获取同一把锁。普通 Store 方法也分别通过 `View` 或 `Update` 执行。

service 装配 Store 时使用 `Coordinate` 包装它；如果传入对象已经实现 `Coordinator`，则直接复用，避免同一后端出现互不相知的嵌套锁。

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
4. 删除现有文件，包括 cache，再写入备份内容；`settings.json` 使用原子
   `0600` 写入。
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

## 部署不变量

- 单进程部署可以依赖 Coordinator 避免备份与普通写入交错。
- 多进程共享 Store 时，维护窗口、写入者停止和存储级快照由外部协调。
- 关键数据仍需独立的存储级备份；应用归档不能替代底层 durability。
- 自定义 Store 必须通过 key、列举和读写契约测试，再用于 share 或恢复场景。
- cache 永远可以丢弃，领域资源和其它非 cache Store 数据才是恢复目标。
