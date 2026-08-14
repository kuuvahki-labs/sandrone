# Vercel + S3 Serverless 部署设计

## 背景

Sandrone 当前以长驻 Go 进程运行。 `internal/app.NewRuntime` 始终创建以
`SANDRONE_DATA_DIR` 为根目录的 `FSStore`，CLI 启动 HTTP listener 和定时刷新
循环，同一个进程同时提供内嵌 Web UI、HTTP API、MCP endpoint 与公开分享。

service 与 domain 层已经依赖窄化的 Store 边界，而不是直接依赖文件系统路径。
当前契约包含 `Read`、`Write`、`Delete`、`List` 和 `Stat`，另有供
`settings.json` 使用的可选 `AtomicWriter`。分享读取已经是纯读操作，Store
也不再暴露 CAS 或限次分享计数。这些边界允许在不修改资源与分享 wire format
的前提下接入对象存储后端。

目标是支持单租户部署：应用运行在 Vercel Functions，权威资源与持久缓存保存在
Cloudflare R2。存储实现使用 S3 API，并继续允许 CLI、Docker 及其他长驻部署
配置兼容的 S3 服务。

截至 2026-08-14，
[Vercel Go Runtime](https://vercel.com/docs/functions/runtimes/go) 仍处于 Beta，
它会从 `api/*.go` 构建一个导出的 HTTP handler。Vercel Functions 存在
[4.5 MB 请求与响应 payload 上限](https://vercel.com/docs/functions/limitations)。
当前 Sandrone 去除符号后的二进制在不带可选 probe tags 时约为 23 MB，带正常
发布 tags 时约为 42.6 MB；内嵌 Web 资源约增加 2.1 MB。两者都低于 Vercel 当前
函数 bundle 上限，但 Vercel profile 会主动关闭测活，因此不携带 probe tags。

## 目标

- 为现有 Store 契约增加一等公民的 S3 后端实现。
- 允许 CLI、Docker 和长驻 `serve` 部署通过环境变量选择 `filesystem` 或
  `s3`，并保持 filesystem 为默认值。
- 增加一个 Vercel Go Function，复用现有 HTTP、MCP、Web UI 与 service handler，
  不自行启动 listener。
- 通过 S3-compatible endpoint 直连 Cloudflare R2，不增加 Cloudflare Worker、
  D1、KV 或 afero S3 adapter。
- 保持 subscription、file、settings、share、backup、cache、HTTP API、MCP 与
  Web 资源格式不变。
- 让 Vercel profile 明确声明并强制执行“定时刷新和所有测活功能不可用”。
- 确保凭据不会进入命令行参数、日志、响应、Store 对象或备份。

## 非目标

- 在 Cloudflare Workers 或 Pages Functions 上运行 Sandrone。
- 增加多租户账号、用户级 namespace、角色、配额或计费。
- 增加数据库、分布式锁、CAS、事务日志或 Worker gateway。
- 在第一版提供 serverless scheduler。
- 为绕过 Vercel 4.5 MB payload 上限而增加另一套大结果或大备份交付协议。
- 保证兼容所有自称支持 S3 API 的产品。受支持的服务必须满足下文定义的操作与
  一致性语义。
- 改变 CLI、Docker、OpenWrt 或其他长驻部署中的测活和定时刷新行为。

## 选定架构

### 存储选择

所有生产 entrypoint 使用统一的存储配置和工厂。后端仅通过环境变量选择：

| 环境变量 | 是否必需 | 默认值 | 含义 |
| --- | --- | --- | --- |
| `SANDRONE_STORAGE_BACKEND` | 否 | `filesystem` | `filesystem` 或 `s3` |
| `SANDRONE_DATA_DIR` | 仅 filesystem | `./data` | 文件系统 Store 根目录 |
| `SANDRONE_S3_ENDPOINT` | s3 | 无 | 绝对 HTTP(S) S3 endpoint |
| `SANDRONE_S3_REGION` | s3 | 无 | 服务 region；R2 使用 `auto` |
| `SANDRONE_S3_BUCKET` | s3 | 无 | 已存在的 bucket 名称 |
| `SANDRONE_S3_PREFIX` | 否 | `sandrone/` | 非空对象 namespace |
| `SANDRONE_S3_FORCE_PATH_STYLE` | 否 | `false` | 是否启用 path-style addressing |
| `SANDRONE_S3_ACCESS_KEY_ID` | s3 | 无 | 显式 access key ID |
| `SANDRONE_S3_SECRET_ACCESS_KEY` | s3 | 无 | 显式 secret access key |
| `SANDRONE_S3_SESSION_TOKEN` | 否 | 空 | 可选的临时 session token |

S3 后端不使用 AWS 默认凭据链。endpoint、region、bucket、prefix、布尔值或凭据
缺失或非法时，runtime 构造直接失败。endpoint 不能包含 user info、query 或
fragment。prefix 规范化为恰好一个结尾斜杠，并且必须是安全的相对 Store
namespace。选择 `s3` 时忽略显式 `--data-dir`；文档继续将其定义为仅
filesystem 生效的选项。

工厂返回现有 internal Store interface。app runtime 与直接 CLI engine 共用该
工厂，因此选择 S3 后，`serve`、CLI 存储资源操作、settings、share、backup
和现有 Store-backed cache 都使用同一个后端。service method 不参与后端选择。

### S3Store 契约

`S3Store` 将每个 canonical Sandrone key 映射为配置 prefix 下的一个对象。
实现只使用 `GetObject`、`PutObject`、`DeleteObject`、`HeadObject` 与
分页 `ListObjectsV2`。

- `Read` 返回完整对象正文，并将服务端 not-found 响应映射为
  `os.ErrNotExist`。
- `Write` 以已知长度上传 bytes 到一个对象。并发写保持现有的
  last-completing-writer-wins 语义。
- `WriteAtomic` 执行相同的单对象 PUT，并忽略文件系统 mode。一次完成的 PUT
  发布完整的 `settings.json` 对象，不模拟 rename。
- `Delete` 先执行 `HeadObject` 再执行 `DeleteObject`，以保持删除缺失 key
  时返回 not-found 的现有契约。
- `Stat` 返回对象 size 和 last-modified time。如果 key 对应的对象不存在，但
  `key/` 下存在后代对象，则返回合成的 directory entry。
- `List("")` 递归列举配置 namespace；空 namespace 返回空列表。非根 prefix
  不存在匹配对象或后代时返回 `os.ErrNotExist`。
- `List` 持续读取 continuation token 直至结束，移除物理 prefix，拒绝不安全
  或重复的逻辑 key，合成 directory entry，并按 key 稳定排序。
- 所有 SDK 调用传递 context cancellation。错误只包含操作名与逻辑 key，不包含
  凭据或对象正文。

配置的 S3 服务必须为上述操作提供兼容的 read-after-write、overwrite、delete 与
list 一致性。Cloudflare 已说明
[R2 的对象读取、写入、删除与列举均为强一致](https://developers.cloudflare.com/r2/reference/consistency/)。
本设计不依赖 ACL、object locking、versioning、multipart upload、presigned URL
或服务商专属 metadata。

现有进程内 Coordinator 仍是唯一的复合操作锁。它可以防止同一进程或 warm
function instance 内部发生交错，但不是跨 Vercel instance 或多个 `serve`
进程的分布式锁。

### Runtime 装配

runtime 构造改为与后端无关：创建一个 Store，以现有 Coordinator 包装，解析已
存储 settings，并把同一个 coordinator 注入 service 与 cache 路径。删除多余的
`Runtime.Engine` instance，因为没有 entrypoint 使用它，而且它当前会在一个
独立装配的 filesystem 上创建第二套 service。

直接 CLI command 也根据所选 Store 构造 engine，不再总是创建 afero
filesystem。保留现有测试 factory hook，但 factory 构造现在允许返回配置或连接
错误。

`sandrone doctor` 继续验证真实部署路径：

- filesystem 模式保留现有 data-directory 检查；
- S3 模式在保留的 doctor prefix 内写入唯一命名对象，依次读取、stat、list、
  delete，并验证删除后的 read 为缺失；部分失败后执行 best-effort cleanup。

doctor 不输出凭据、endpoint user info 或对象正文。

### Vercel profile

`api/index.go` 导出唯一的 Vercel `http.HandlerFunc`。通过 `sync.Once`
initializer，在每个 warm function instance 中只构造一次与后端无关的 runtime
和现有 HTTP server handler。初始化失败时返回脱敏 JSON 500 响应，并记录不含
secret 的分类错误。

该 handler：

- 服务任何请求前都要求 `SANDRONE_TOKEN` 非空；
- 挂载现有 Web UI、HTTP API、公开分享与 stateless MCP handler；
- 不调用 `ListenAndServe`，也不启动定时刷新循环；
- 通过 runtime option 禁用 scheduler 与 probe capability；
- 在 warm instance 内复用已初始化的 S3 client 和 HTTP transport；
- 依赖 request context 传播 cancellation 与 Vercel invocation deadline。

`vercel.json` 配置该 Go Function，将所有应用路径 rewrite 到 `/api/index.go`，并把
`maxDuration` 设为 60 秒。其 `GO_BUILD_FLAGS` 精确为
`-ldflags '-s -w'`，保留 stripped linker output，但不启用
`probe_singbox`、`with_quic`、`with_wireguard` 或 `with_utls`。

部署到 Vercel 时，使用 R2 参数设置通用 S3 环境变量：

```text
SANDRONE_STORAGE_BACKEND=s3
SANDRONE_S3_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
SANDRONE_S3_REGION=auto
SANDRONE_S3_BUCKET=<bucket>
SANDRONE_S3_PREFIX=sandrone/
```

Preview 与 Production deployment 必须使用不同的 prefix 或 bucket，不能意外
共享同一个 namespace。

### Serverless capability 策略

Vercel runtime 注入 disabled probe engine。它的 backend summary 为空，执行时
返回现有 `probe_backend_unavailable` domain error。因此 capability 为：

- `probe.enabled=false`；
- `core.mihomo=false`；
- `core.sing_box=false`；
- `scheduler.enabled=false`。

Web UI 根据这些 capability 隐藏 probe defaults、测活控件、probe processor
选项、定时刷新配置和状态轮询。直接调用 `/v1/probe`、MCP probe tool，以及执行
已存储或 inline probe processor 时不能静默跳过，而是返回
`probe_backend_unavailable`。共享 settings 中可以保留既有定时刷新配置，但
serverless 不运行 scheduler，UI 也不暴露该配置。

长驻 runtime 继续保留其已编译的 probe backend、capability report、probe
processor 行为与定时刷新循环。

## 请求与数据流

1. Vercel 将所有路径路由到唯一 Go Function。
2. warm-instance initializer 校验环境配置，创建 S3 client 与 S3Store，解析
   settings，并构造现有 HTTP、MCP 与 Web handler。
3. 公开 version、conversion、share 和 Web 路径沿用现有鉴权策略；管理 HTTP 与
   MCP 路径要求现有 bearer token。
4. service 的资源和 cache 操作使用 canonical Store key。
5. S3Store 添加 deployment namespace，执行对应的 S3 请求。
6. service 返回现有 response shape，不向客户端暴露 S3 URL、object key、服务商
   响应或凭据。

CLI 与 Docker 在启动后经过同一 Store 路径，但继续使用现有进程生命周期；
`serve` 继续保留现有 listener 和 scheduler。

## 失败与一致性边界

- 存储配置非法或不完整时 fail closed，不启动可处理鉴权管理流量的 runtime。
- S3 not-found error 保持现有 HTTP 与 service not-found 行为。
- S3 鉴权、网络、throttling 或服务商错误保持为操作失败；cache failure 继续遵循
  现有 best-effort cache 语义。
- function cancellation 可能使服务商请求结果处于未知状态；后续强一致
  `Read` 或 `Stat` 用于确定已存储状态。
- 复合 backup restore 仍是 best-effort 且非事务操作。在任何共享 S3 namespace
  中执行时，都要求进入停止其他 writer 的维护窗口；Vercel 不增加跨 instance
  锁。
- 超过 4.5 MB 的 Vercel 请求与响应由平台拒绝，即使 Sandrone 的 backup 与
  render 上限更大。第一版只记录这一 profile-specific 限制，不修改 API wire
  format。
- 持久 Store-backed cache 可能产生大量小 S3 object 与请求。现有 TTL、失效和
  backup 排除 cache 的规则保持不变。如果请求成本高于复用收益，operator 可通过
  现有 settings 关闭 result-cache TTL。
- 推荐使用 R2 Standard storage class。其 free tier 和 operation class 见
  [Cloudflare R2 pricing](https://developers.cloudflare.com/r2/pricing/)，但价格
  不属于应用契约。

## 安全

- S3 凭据只接受环境变量，不提供 flag，也不写入项目 settings。
- Vercel 强制要求非空 Sandrone bearer token。Preview 与 Production 分别配置
  secret。
- S3 token 应限制为单个 bucket 的 object read/write 权限，不需要 bucket
  administration、CORS、lifecycle 或 public-bucket 权限。
- bucket 保持私有。Sandrone 公开分享通过应用的 Store 路径动态渲染，不暴露为
  public S3 object。
- 日志只使用逻辑操作名和脱敏错误。测试必须断言 access key、secret key、token、
  含 user info 的 endpoint、资源正文和订阅 URL 不会被输出。

## 验证

### Store 契约测试

针对 FSStore 与 mock S3 client 运行同一组 Store contract suite，覆盖：

- canonical 与非法 key；
- read/write/overwrite/delete/not-found 行为；
- atomic settings write；
- 根与非根 list、分页、稳定排序、重复拒绝和 virtual directory；
- object 与 virtual directory 的 stat；
- context cancellation 与脱敏的服务商错误。

增加一个针对 S3-compatible 测试 endpoint 的 opt-in integration suite。它必须
运行相同契约，并覆盖 settings、resource、cache、share、backup export 与 backup
restore 流程。R2 凭据仅用于外部 deployment smoke test，绝不提交到仓库。

### Runtime 与 CLI 测试

- filesystem 仍为默认值，data-dir 行为不变。
- 每个必需 S3 环境变量，以及非法 boolean、endpoint、prefix，都产生字段明确且
  不泄露 secret 的启动错误。
- 选择 S3 后，CLI resource operation 与 `serve` 都使用 S3。
- S3 doctor 验证 write/read/stat/list/delete，并清理测试对象。
- 长驻 S3 `serve` 仍暴露 scheduler 与已编译 probe capability。

### Vercel profile 测试

- 导出的 handler 只初始化一次，提供内嵌 Web 资源，保留全部现有 route，并拒绝
  缺少 `SANDRONE_TOKEN` 的配置。
- UI capability 报告 scheduler 和所有 probe/core feature 均为 disabled。
- `/v1/probe`、MCP probe execution 与 probe processor 返回
  `probe_backend_unavailable`。
- Web 测试证明 capability disabled 时，不显示 probe settings、probe processor
  选项、定时刷新与状态轮询。
- `vercel.json` contract test 锁定单一 function、全路径 rewrite、不含 probe
  tags 的精确 stripped build flags，以及 60 秒 duration。
- 使用独立 R2 prefix 的真实 Vercel Preview Deployment 验证 version、Web 资源、
  鉴权 CRUD、跨 cold start 的 settings 持久化、公开分享、conversion、MCP，以及
  小于 4.5 MB 的 backup round trip。

先运行相关 Go 与 Web 窄测，再运行全仓 `make check`。capability rendering
发生变化时，还要运行现有 Web `pnpm test:run`、`typecheck` 与 `lint`
门禁。Preview smoke test 是 Vercel runtime 与 R2 互操作的最终证明。

## 文档与发布

- 扩展存储架构，说明 S3Store 语义、namespace 规则、服务商一致性要求和多进程
  restore 限制。
- 扩展 CLI 参考，说明 backend/S3 环境变量和 S3 doctor 行为。
- 增加 Vercel + R2 部署指南，包含 bucket/token 设置、环境变量、
  Preview/Production namespace 隔离、smoke check、disabled capability、
  4.5 MB 限制，以及回滚到上一 deployment 的步骤。
- Docker 与 README quick start 继续默认使用 filesystem storage。
- 不自动迁移现有 filesystem 数据。operator 在维护窗口中从 filesystem deployment
  导出 backup，再恢复到 S3 deployment，同时受目标平台 payload limit 约束。

当 filesystem 默认行为保持不变、共享 S3 Store 通过 contract 与 integration
suite、Vercel profile 按设计隐藏并拒绝 scheduler/probe 功能、所有本地门禁通过，
且真实 Preview Deployment 能通过 R2 持久化 Sandrone 核心工作流时，功能才算
完成。
