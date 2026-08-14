# Vercel + S3 Serverless 部署实施计划

> **供执行型代理使用：** REQUIRED SUB-SKILL: 使用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans` 逐任务实施本计划。所有步骤使用 checkbox 跟踪。

**目标：** 让 Sandrone 在保持 filesystem 默认部署不变的前提下，支持 CLI、Docker、长驻 `serve` 与 Vercel Go Function 共用通用 S3-compatible Store；Vercel 以 Cloudflare R2 为主要目标，并强制关闭定时刷新和所有测活执行能力。

**架构：** 在现有 `internal/store.Store` 边界下增加直接使用 AWS SDK v2 的 `S3Store`，由 `internal/app` 统一解析环境变量并创建后端。长驻入口继续启动 listener、scheduler 和编译进来的 probe backend；`api/index.go` 只导出复用现有 HTTP/MCP/Web 装配的 handler，并注入 disabled probe engine 与关闭 scheduler 的 runtime option。

**技术栈：** Go 1.25、AWS SDK for Go v2、Cloudflare R2 S3 API、Vercel Go Runtime、`net/http`、React 19、TypeScript、Vitest、Playwright。

## 全局约束

- filesystem 仍是默认后端；未设置 `SANDRONE_STORAGE_BACKEND` 时，现有 CLI、Docker 和 `serve` 行为不得改变。
- S3 是产品级通用兼容契约，R2 是第一套文档化并真实验证的服务；不得引入 D1、KV、Cloudflare Worker gateway 或 `afero-s3`。
- 所有 S3 配置只来自环境变量；不得增加 S3 flags，也不得调用 AWS 默认凭据链。
- 不修改 Store key、资源 JSON、share URL、backup 格式、HTTP/MCP wire format 或公开嵌入 Store 接口。
- Vercel profile 必须要求非空 `SANDRONE_TOKEN` 和 `s3` 后端；不得退化到 Vercel 临时文件系统。
- Vercel profile 的 `probe.enabled`、`core.mihomo`、`core.sing_box`、`scheduler.enabled` 均为 `false`。
- `/v1/probe`、MCP probe tool、probe processor 和脚本中的 probe 调用不得静默跳过，统一返回 `probe_backend_unavailable`。
- Web 只隐藏 capability 不可用的入口，不删除或重写已经存储的 probe/scheduler 设置。
- S3 restore 继续是 best-effort；不增加跨进程锁、CAS、事务或分布式协调。
- Vercel 4.5 MB 请求/响应限制只写入部署契约，不为其增加旁路传输协议。
- 测试、日志和错误不得输出 access key、secret、session token、带 user info 的 endpoint、对象正文或真实订阅 URL。
- 实施时保留工作树中用户已有改动；每个任务先跑窄测，最终跑完整 Go/Web 门禁。
- 计划中的 commit 步骤只有在用户明确授权提交时执行；否则保留经过验证的工作树改动。

---

### 任务 1：引入 S3 SDK，并锁定配置规范化契约

**文件：**

- 修改：`go.mod`
- 修改：`go.sum`
- 新建：`internal/store/s3_config.go`
- 新建：`internal/store/s3_config_test.go`

**接口：**

- 新增：`store.S3Config`
- 新增：`func NormalizeS3Config(S3Config) (S3Config, error)`
- 产出：不含默认凭据链、可供 app factory 和 `S3Store` 共同使用的规范配置。

- [ ] **步骤 1：加入固定版本依赖**

运行：

```bash
go get \
  github.com/aws/aws-sdk-go-v2/credentials@v1.19.35 \
  github.com/aws/aws-sdk-go-v2/service/s3@v1.107.1 \
  github.com/aws/smithy-go@v1.27.7
go mod tidy
```

预期：`go.mod` 只加入 S3 client、显式 credentials provider 及其传递依赖；生产代码不导入 `github.com/aws/aws-sdk-go-v2/config`。

- [ ] **步骤 2：先写配置失败测试**

在 `s3_config_test.go` 覆盖：

```go
func TestNormalizeS3Config(t *testing.T)
func TestNormalizeS3ConfigRejectsMissingRequiredField(t *testing.T)
func TestNormalizeS3ConfigRejectsUnsafeEndpoint(t *testing.T)
func TestNormalizeS3ConfigRejectsUnsafePrefix(t *testing.T)
func TestNormalizeS3ConfigDoesNotExposeSecrets(t *testing.T)
```

表驱动断言至少包括：

- endpoint 只接受绝对 `http`/`https` URL；拒绝 user info、query、fragment 和空 host；
- region、bucket、access key ID、secret access key 都必须非空；
- prefix 必须是非空安全相对 namespace，`sandrone`、`sandrone/` 均规范化为 `sandrone/`；
- `/sandrone`、`../sandrone`、`sandrone//nested`、反斜杠和空 prefix 被拒绝；
- 任一错误字符串都不包含传入的 access key、secret 或 session token。

运行：

```bash
go test ./internal/store -run 'TestNormalizeS3Config' -count=1
```

预期：FAIL，因为配置类型和规范化函数尚不存在。

- [ ] **步骤 3：实现最小配置类型与规范化**

使用以下字段，不向 `S3Config` 加入日志标签或 `String()`：

```go
type S3Config struct {
	Endpoint         string
	Region           string
	Bucket           string
	Prefix           string
	ForcePathStyle   bool
	AccessKeyID      string
	SecretAccessKey  string
	SessionToken     string
}
```

`NormalizeS3Config` 只返回字段名明确的错误，例如 `SANDRONE_S3_BUCKET is required`；错误不得格式化整个 config。prefix 使用 `CleanKey` 校验去掉结尾斜杠后的逻辑 namespace，再补回恰好一个 `/`。

- [ ] **步骤 4：运行配置测试和依赖检查**

```bash
go test ./internal/store -run 'TestNormalizeS3Config' -count=1
go mod tidy
git diff --check
```

预期：全部 PASS，且 `git diff --check` 无输出。

- [ ] **步骤 5：按授权决定是否提交**

如用户已明确授权 commit：

```bash
git add go.mod go.sum internal/store/s3_config.go internal/store/s3_config_test.go
git commit -m "feat(store): define S3 backend configuration"
```

---

### 任务 2：实现满足现有 Store 语义的 S3Store

**文件：**

- 新建：`internal/store/s3.go`
- 新建：`internal/store/s3_test.go`
- 修改：`internal/store/store_test.go`

**接口：**

- 新增：`func NewS3Store(context.Context, S3Config) (*S3Store, error)`
- 新增：`S3Store` 对 `Store` 和 `AtomicWriter` 的完整实现。
- 内部 seam：窄化 `s3API`，只包含 `GetObject`、`PutObject`、`DeleteObject`、`HeadObject`、`ListObjectsV2`。

- [ ] **步骤 1：先写 mock S3 契约测试**

在 `s3_test.go` 使用内存 fake client，测试名至少包括：

```go
func TestS3StoreReadWriteOverwriteAndDelete(t *testing.T)
func TestS3StoreMapsObjectNotFound(t *testing.T)
func TestS3StoreDeleteChecksExistence(t *testing.T)
func TestS3StoreWriteAtomicUsesSinglePut(t *testing.T)
func TestS3StoreListRootAcrossPages(t *testing.T)
func TestS3StoreListPrefixAndVirtualDirectories(t *testing.T)
func TestS3StoreListRejectsUnsafeOrDuplicateLogicalKeys(t *testing.T)
func TestS3StoreStatObjectAndVirtualDirectory(t *testing.T)
func TestS3StorePropagatesContextCancellation(t *testing.T)
func TestS3StoreErrorsDoNotLeakSecretsOrBodies(t *testing.T)
```

mock interface 使用 SDK 的真实方法签名：

```go
type s3API interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}
```

运行：

```bash
go test ./internal/store -run 'TestS3Store' -count=1
```

预期：FAIL，因为 `S3Store` 尚未实现。

- [ ] **步骤 2：用显式凭据创建 SDK client**

`NewS3Store` 必须：

- 先调用 `NormalizeS3Config`；
- 直接构造 `aws.Config{Region: ..., Credentials: credentials.NewStaticCredentialsProvider(...)}`；
- 通过 `s3.NewFromConfig` 设置 `BaseEndpoint` 和 `UsePathStyle`；
- 不调用 `config.LoadDefaultConfig`，不读取 `AWS_*`、EC2/ECS metadata 或 shared credentials；
- 保存 bucket 与规范 prefix，不把 credentials 保存到 `S3Store` 字段。

保留一个仅供 package test 使用的 constructor seam：

```go
func newS3Store(client s3API, bucket, prefix string) *S3Store
```

- [ ] **步骤 3：实现单对象操作**

实现规则：

- `Read`：`CleanKey` 后 `GetObject`，完整读取并关闭 body；`NoSuchKey`/`NotFound` 映射为 `os.ErrNotExist`；
- `Write`：单次 `PutObject`，设置已知 `ContentLength`，last-completing-writer-wins；
- `WriteAtomic`：直接复用单次 PUT，忽略 `fs.FileMode`，不得模拟临时 key + copy + delete；
- `Delete`：先 `HeadObject`，存在后再 `DeleteObject`，保持删除缺失 key 返回 not-found；
- 所有 SDK 调用使用调用方 context。

错误包装只包含操作和逻辑 key，例如 `s3 read subscriptions/demo.json: ...`，不包含物理 endpoint、credentials 或正文。

- [ ] **步骤 4：实现 List 和 Stat**

保持与 `FSStore` 一致：

- `List("")` 对物理 namespace 做全量分页；无对象返回空 slice；
- 非根 prefix 先检查同名对象；如果是对象，只返回该对象 entry；否则只列举 `prefix/` 后代；
- 非根 prefix 不存在对象或后代时返回 `os.ErrNotExist`；
- 循环 continuation token 直到 `IsTruncated=false`；
- 移除物理 prefix 后对每个逻辑 key 执行 `CleanKey`，拒绝重复 key；
- 为后代合成所有中间 directory entry，但不返回查询 root 本身；
- 最终按 `Entry.Key` 稳定排序；
- `Stat` 先 `HeadObject`，不存在时用 `ListObjectsV2(MaxKeys=1, Prefix=key+"/")` 判断虚拟目录；
- 对象 entry 使用 S3 size 和 last-modified，虚拟目录 size 为 0、`IsDir=true`。

- [ ] **步骤 5：把共同 Store contract 抽成测试 helper**

在 `store_test.go` 中抽取：

```go
func runStoreContract(t *testing.T, newStore func(t *testing.T) Store)
```

让现有 `FSStore` 与 mock `S3Store` 都运行 key、CRUD、List、Stat、not-found 与 `AtomicWriter` 契约。保留 FS 专属的 rename/mode 测试和 S3 专属分页/错误映射测试，避免为了共享 helper 删除有价值的覆盖。

- [ ] **步骤 6：运行 Store 全包测试**

```bash
go test ./internal/store -count=1
go vet ./internal/store
git diff --check
```

预期：全部 PASS。

- [ ] **步骤 7：按授权决定是否提交**

如已授权：

```bash
git add internal/store/s3.go internal/store/s3_test.go internal/store/store_test.go
git commit -m "feat(store): add S3-compatible backend"
```

---

### 任务 3：统一 app 存储选择与 runtime 装配

**文件：**

- 新建：`internal/app/storage.go`
- 新建：`internal/app/storage_test.go`
- 修改：`internal/app/runtime.go`
- 修改：`internal/app/runtime_test.go`
- 修改：`internal/app/settings_test.go`

**接口：**

- 新增：`StorageBackend`、`StorageConfig`、`StorageConfigFromEnv`
- 新增：`func NewStore(context.Context, string, StorageConfig) (store.Store, error)`
- 新增：`func NewRuntimeContext(context.Context, Config, *slog.Logger, ...RuntimeOption) (*Runtime, error)`
- 保留：`NewRuntime(Config, *slog.Logger)` 作为默认长驻行为 wrapper。
- 删除：未被使用的 `Runtime.Engine`。

- [ ] **步骤 1：先写环境配置矩阵测试**

在 `storage_test.go` 覆盖：

```go
func TestStorageConfigFromEnvDefaultsToFilesystem(t *testing.T)
func TestStorageConfigFromEnvBuildsS3Config(t *testing.T)
func TestStorageConfigFromEnvRejectsUnsupportedBackend(t *testing.T)
func TestStorageConfigFromEnvRejectsInvalidBoolean(t *testing.T)
func TestStorageConfigFromEnvRequiresEveryS3Field(t *testing.T)
func TestStorageConfigFromEnvDoesNotLeakSecrets(t *testing.T)
func TestNewStoreUsesFilesystemDataDir(t *testing.T)
```

环境变量常量集中定义在 `internal/app/storage.go`：

```go
const (
	EnvStorageBackend    = "SANDRONE_STORAGE_BACKEND"
	EnvS3Endpoint        = "SANDRONE_S3_ENDPOINT"
	EnvS3Region          = "SANDRONE_S3_REGION"
	EnvS3Bucket          = "SANDRONE_S3_BUCKET"
	EnvS3Prefix          = "SANDRONE_S3_PREFIX"
	EnvS3ForcePathStyle  = "SANDRONE_S3_FORCE_PATH_STYLE"
	EnvS3AccessKeyID     = "SANDRONE_S3_ACCESS_KEY_ID"
	EnvS3SecretAccessKey = "SANDRONE_S3_SECRET_ACCESS_KEY"
	EnvS3SessionToken    = "SANDRONE_S3_SESSION_TOKEN"
)
```

`SANDRONE_S3_PREFIX` 默认 `sandrone/`，`SANDRONE_S3_FORCE_PATH_STYLE` 默认 `false`。filesystem 模式不要求也不验证无关的 S3 字段。

运行：

```bash
go test ./internal/app -run 'Test(StorageConfig|NewStore)' -count=1
```

预期：FAIL。

- [ ] **步骤 2：实现后端工厂**

使用以下模型：

```go
type StorageBackend string

const (
	StorageFilesystem StorageBackend = "filesystem"
	StorageS3         StorageBackend = "s3"
)

type StorageConfig struct {
	Backend StorageBackend
	S3      store.S3Config
}
```

`NewStore` 在 filesystem 模式使用现有 `afero.BasePathFs + FSStore`，在 S3 模式调用 `store.NewS3Store`。它返回 raw Store，由调用方统一 `Coordinate`；不得把 backend 选择下沉到 service。

- [ ] **步骤 3：先写 runtime profile 测试**

新增测试证明：

```go
func TestNewRuntimeDefaultsToFilesystemSchedulerAndProbe(t *testing.T)
func TestNewRuntimeUsesConfiguredStoreForSettings(t *testing.T)
func TestNewRuntimeContextPropagatesCancellation(t *testing.T)
func TestRuntimeDoesNotConstructDuplicateEngine(t *testing.T)
```

第二个测试使用 fake Store factory 或内存 afero：预先写入 `settings.json`，断言 runtime 读取的 stored/effective settings 来自同一个 Store。

- [ ] **步骤 4：重构 runtime 构造但保持默认行为**

`Config` 增加 `Storage StorageConfig`。新增 option 结构：

```go
type RuntimeOption func(*runtimeOptions)

func WithSchedulerEnabled(bool) RuntimeOption
func WithProbeEngine(service.ProbeEngine) RuntimeOption
func WithStoreFactory(func(context.Context, string, StorageConfig) (store.Store, error)) RuntimeOption
```

`NewRuntime` 调用 `NewRuntimeContext(context.Background(), cfg, logger)`；后者按顺序：

1. 应用 filesystem/storage/runtime 默认值；
2. 创建一个 raw Store 并 `Coordinate` 一次；
3. 用同一个 coordinator 创建 `SettingsStore` 并解析存储设置；
4. 完成现有 `Validate`；
5. 创建 logger；
6. 把同一个 coordinator、settings、logger、scheduler 开关和可选 prober 注入 `service.New`。

从 `Runtime` 删除 `Engine *sandrone.Engine` 及 `pkg/sandrone` import。`NewRuntimeContext` 任何失败都直接返回，不创建第二套 filesystem service。

- [ ] **步骤 5：运行 app 测试**

```bash
go test ./internal/app -count=1
go test ./internal/entry/httpapi ./internal/entry/mcpapi -count=1
git diff --check
```

预期：全部 PASS，现有 HTTP/MCP runtime helper 仍可不改调用方式。

- [ ] **步骤 6：按授权决定是否提交**

如已授权：

```bash
git add internal/app
git commit -m "refactor(app): assemble runtime from selected store"
```

---

### 任务 4：让所有 CLI 命令和 doctor 使用所选 Store

**文件：**

- 修改：`internal/entry/cli/env.go`
- 修改：`internal/entry/cli/root.go`
- 修改：`internal/entry/cli/commands.go`
- 修改：`internal/entry/cli/serve.go`
- 修改：`internal/entry/cli/doctor.go`
- 修改：`internal/entry/cli/cli_test.go`

**接口：**

- 生产 `engineFactory` 接受 context、`app.StorageConfig` 与 data dir，并返回 engine 与对应 Store。
- `doctor` 输出增加 backend 级状态；filesystem 保留 data-dir 检查，S3 执行真实 Store round trip。

- [ ] **步骤 1：先写 CLI 后端选择失败测试**

在 `cli_test.go` 增加：

```go
func TestCLICommandsDefaultToFilesystemStorage(t *testing.T)
func TestCLICommandsPassS3StorageToEngineFactory(t *testing.T)
func TestServePassesS3StorageToRuntimeFactory(t *testing.T)
func TestCLIRejectsInvalidStorageBeforeExecutingCommand(t *testing.T)
func TestDataDirFlagIsIgnoredByS3Factory(t *testing.T)
```

测试覆盖至少一个需要 Store 的直接命令（`file render` 或 `validate --file`）和 `serve`。不要只测纯 `convert`，因为它无法证明资源 Store 被替换。

运行：

```bash
go test ./internal/entry/cli -run 'Test(CLICommands|ServePasses|DataDirFlag)' -count=1
```

预期：FAIL，当前 `newEngine` 总是创建 afero filesystem。

- [ ] **步骤 2：重构 CLI engine factory**

把内部 factory 改为：

```go
type engineFactory func(
	context.Context,
	app.StorageConfig,
	string,
) (engine, store.Store, error)
```

生产 `newEngine` 调用 `app.NewStore`，再用同一 Store 创建 `service.New(service.WithStore(...))`。更新 `commands.go` 的所有七个 factory 调用点，使配置错误在读取输入或写输出前返回。

同步更新测试用 `WithEngineFactory` 和 `WithRuntimeFactory` seam；测试 factory 必须能观察 storage config，但不得要求真实 S3 网络。

- [ ] **步骤 3：把 storage config 注入 serve runtime**

`newServeRuntime` 在 flag 解析完成后调用 `app.StorageConfigFromEnv(cfg.env)`，并设置 `app.Config.Storage`。`--data-dir` 继续只影响 filesystem；S3 模式不新增 flag，不把 storage 字段写入 `OverrideSources` 或项目 settings。

- [ ] **步骤 4：先写 doctor Store round-trip 测试**

新增：

```go
func TestDoctorChecksFilesystemStorage(t *testing.T)
func TestDoctorChecksS3StoreRoundTrip(t *testing.T)
func TestDoctorCleansUpAfterPartialStoreFailure(t *testing.T)
func TestDoctorStorageErrorDoesNotLeakObjectBody(t *testing.T)
```

`doctorResult` 调整为：

```go
type doctorResult struct {
	OK              bool                `json:"ok"`
	StorageBackend  string              `json:"storage_backend"`
	StorageOK       bool                `json:"storage_ok"`
	StorageError    string              `json:"storage_error,omitempty"`
	DataDir         string              `json:"data_dir,omitempty"`
	DataDirWritable bool                `json:"data_dir_writable,omitempty"`
	DataDirError    string              `json:"data_dir_error,omitempty"`
	ParseFormats    []doctorFormatCheck `json:"parse_formats"`
	RenderFormats   []doctorFormatCheck `json:"render_formats"`
}
```

- [ ] **步骤 5：实现通用 Store doctor**

filesystem 保留原来的创建/写入/删除检查。S3 使用 `_doctor/<随机值>.check`，依次验证：

1. `Write("ok")`；
2. `Read` 内容相等；
3. `Stat` size 正确；
4. `List("_doctor")` 包含该 key；
5. `Delete` 成功；
6. 删除后的 `Read` 满足 `errors.Is(err, os.ErrNotExist)`。

从首次成功写入起 defer best-effort cleanup。错误只进入 `storage_error`，不打印正文、credentials 或完整 endpoint。

- [ ] **步骤 6：运行 CLI 全包测试**

```bash
go test ./internal/entry/cli -count=1
go vet ./internal/entry/cli
git diff --check
```

预期：全部 PASS；原有 CLI 输出测试除新增 doctor 字段外保持不变。

- [ ] **步骤 7：按授权决定是否提交**

如已授权：

```bash
git add internal/entry/cli
git commit -m "feat(cli): select storage backend from environment"
```

---

### 任务 5：增加 disabled probe engine 与 serverless capability 策略

**文件：**

- 新建：`internal/probe/disabled.go`
- 新建：`internal/probe/disabled_test.go`
- 修改：`internal/service/probe.go`
- 修改：`internal/service/probe_test.go`
- 修改：`internal/service/ui_capabilities_test.go`
- 修改：`internal/app/runtime_test.go`

**接口：**

- 新增：`func probe.NewDisabled() *DisabledEngine`
- 新增内部可用性接口：`ProbeAvailable() bool`
- 产出：capability 为空且任何 probe 执行都快速返回 `CodeProbeBackendUnavailable` 的 runtime 依赖。

- [ ] **步骤 1：先写 disabled engine 与 service 快速失败测试**

新增：

```go
func TestDisabledEngineHasNoBackendsOrCore(t *testing.T)
func TestDisabledEngineReturnsBackendUnavailable(t *testing.T)
func TestServiceDisabledProbeFailsBeforeResolvingInput(t *testing.T)
func TestServiceDisabledProbeProcessorReturnsBackendUnavailable(t *testing.T)
func TestDisabledProbeAndSchedulerUICapabilities(t *testing.T)
```

第三个测试给出本来会触发远程抓取或 Store 读取的 input，并断言依赖未被调用，从而证明 serverless 不会先做昂贵工作再失败。processor 测试使用一个 inline 或已存储 subscription，包含 nodes-stage `probe` processor。

运行：

```bash
go test ./internal/probe ./internal/service -run 'Test(Disabled|ServiceDisabled)' -count=1
```

预期：FAIL。

- [ ] **步骤 2：实现 DisabledEngine**

实现：

```go
type DisabledEngine struct{}

func NewDisabled() *DisabledEngine
func (*DisabledEngine) ProbeAvailable() bool
func (*DisabledEngine) BackendSummary() []domain.ProbeBackendSummary
func (*DisabledEngine) SelectCore(domain.ProbeRequest, []domain.NodeIR) (string, bool)
func (*DisabledEngine) Probe(context.Context, domain.ProbeRequest, []domain.NodeIR, ...Payload) (*domain.ProbeResult, error)
```

`ProbeAvailable` 返回 `false`，summary 返回空 slice，`SelectCore` 返回 `"", false`，`Probe` 返回：

```go
domain.NewError(domain.CodeProbeBackendUnavailable, "probe backend is not available")
```

- [ ] **步骤 3：在 service 边界快速失败**

在 `Service.Probe` 的 nil 检查之后、默认值/输入解析/cache 之前检查：

```go
type probeAvailability interface {
	ProbeAvailable() bool
}
```

只有实现该接口且返回 `false` 的 engine 才快速失败；普通 `probe.Engine` 和现有测试 fake 的行为不变。processor 与脚本 probe 最终都经过 `Service.Probe`，不在 entrypoint 重复策略。

- [ ] **步骤 4：验证 capability 与长驻默认值**

用 `app.NewRuntimeContext` 注入：

```go
app.WithProbeEngine(probe.NewDisabled())
app.WithSchedulerEnabled(false)
```

断言四项 capability 全为 disabled；另保留一个默认 `NewRuntime` 测试，证明 filesystem/CLI/Docker 长驻 runtime 的 TCP probe 与 scheduler capability 仍启用。

- [ ] **步骤 5：运行相关测试**

```bash
go test ./internal/probe ./internal/service ./internal/app -count=1
git diff --check
```

预期：全部 PASS。

- [ ] **步骤 6：按授权决定是否提交**

如已授权：

```bash
git add internal/probe internal/service/probe.go internal/service/probe_test.go internal/service/ui_capabilities_test.go internal/app/runtime_test.go
git commit -m "feat(app): disable probe in serverless profile"
```

---

### 任务 6：增加单一 Vercel Go Function 与配置契约

**文件：**

- 新建：`api/index.go`
- 新建：`api/index_test.go`
- 新建：`vercel.json`

**接口：**

- 导出：`func Handler(http.ResponseWriter, *http.Request)`
- 内部：`buildHandler` 接受 getenv/runtime factory seam；生产初始化由 `sync.OnceValues` 缓存。

- [ ] **步骤 1：先写 Vercel 配置 contract test**

在 `api/index_test.go` 读取 `../vercel.json`，断言：

- 只有 `api/index.go` 一个 function；
- `maxDuration` 精确为 `60`；
- `/(.*)` 全路径 rewrite 到 `/api/index.go`；
- `GO_BUILD_FLAGS` 精确为 `-ldflags '-s -w'`；
- build flags 不包含 `probe_singbox`、`with_quic`、`with_wireguard`、`with_utls` 或任意 `-tags`。

测试名：

```go
func TestVercelConfigContract(t *testing.T)
```

运行：

```bash
go test ./api -run TestVercelConfigContract -count=1
```

预期：FAIL，因为配置文件尚不存在。

- [ ] **步骤 2：先写 handler 装配测试**

新增：

```go
func TestBuildHandlerRequiresToken(t *testing.T)
func TestBuildHandlerRequiresS3Backend(t *testing.T)
func TestBuildHandlerPassesServerlessRuntimeOptions(t *testing.T)
func TestBuildHandlerServesVersionAndCapabilities(t *testing.T)
func TestBuildHandlerProbeReturnsBackendUnavailable(t *testing.T)
func TestInitializationErrorResponseIsSanitized(t *testing.T)
```

测试 factory 可在内部把捕获到的生产 S3 config 替换为 `t.TempDir()` filesystem runtime，以避免真实网络；但必须断言传给 factory 的原始 config 是 S3，且 runtime options 产生 disabled probe/scheduler capability。

HTTP probe 测试带 bearer token 和合法 inline node input，断言 domain error code 为 `probe_backend_unavailable`。初始化错误响应只允许固定分类消息，不得包含 factory 返回的 secret marker。

- [ ] **步骤 3：实现 warm-instance 单例 handler**

结构保持：

```go
var productionHandler = sync.OnceValues(newProductionHandler)

func Handler(w http.ResponseWriter, r *http.Request) {
	h, err := productionHandler()
	if err != nil {
		writeInitializationError(w)
		return
	}
	h.ServeHTTP(w, r)
}
```

`newProductionHandler`/`buildHandler` 必须：

1. 校验 `SANDRONE_TOKEN` 非空；
2. `app.StorageConfigFromEnv` 并拒绝非 `s3` backend；
3. 构造 `app.Config`，保留现有 MCP 默认 path、1 MiB output limit 与 log level；
4. 调用 `app.NewRuntimeContext`，注入 `probe.NewDisabled()` 和 `WithSchedulerEnabled(false)`；
5. 创建 `mcpapi.New(rt).Handler()`；
6. 使用 `httpapi.New(rt, WithMCP(...), WithWebUI(...)).Handler()`；
7. 不调用 `Run`、`ListenAndServe` 或 `RunScheduledRefresh`。

请求 context 只用于请求本身；warm runtime 初始化使用有界独立 context，SDK 操作继续使用每个请求传入的 context。

- [ ] **步骤 4：写入 vercel.json**

使用精确配置：

```json
{
  "$schema": "https://openapi.vercel.sh/vercel.json",
  "build": {
    "env": {
      "GO_BUILD_FLAGS": "-ldflags '-s -w'"
    }
  },
  "functions": {
    "api/index.go": {
      "maxDuration": 60
    }
  },
  "rewrites": [
    {
      "source": "/(.*)",
      "destination": "/api/index.go"
    }
  ]
}
```

- [ ] **步骤 5：增加 MCP/profile 回归**

复用 `internal/entry/mcpapi` 的 in-memory helper，通过同一 disabled runtime 调用 `sandrone_probe_nodes`，断言结构化 domain code 为 `probe_backend_unavailable`。不要为 Vercel fork 一套 MCP tool registry。

- [ ] **步骤 6：运行 handler 与现有入口测试**

```bash
GOFLAGS=-mod=readonly go test ./api ./internal/entry/httpapi ./internal/entry/mcpapi -count=1
GOFLAGS=-mod=readonly go build -ldflags '-s -w' ./api
git diff --check
```

预期：全部成功，且没有依赖 probe release tags。

- [ ] **步骤 7：按授权决定是否提交**

如已授权：

```bash
git add api vercel.json internal/entry/mcpapi
git commit -m "feat(vercel): add serverless HTTP entrypoint"
```

---

### 任务 7：锁定 Web 对不可用测活与 scheduler 的隐藏行为

**文件：**

- 修改：`web/app/features/subscriptions/components/processor-builder.test.tsx`
- 修改：`web/app/features/settings/pages/settings-detail-pages.test.tsx`
- 修改：`web/app/features/settings/data/settings-data-hooks.test.tsx`
- 仅在测试暴露缺口时修改：
  - `web/app/features/subscriptions/components/processor-builder.tsx`
  - `web/app/features/settings/sections/runtime-settings-section.tsx`
  - `web/app/features/settings/pages/settings-runtime-page.tsx`
  - `web/app/features/settings/data/use-scheduled-refresh-status.ts`

**接口：**

- 消费现有 `/v1/capabilities/ui`；不新增 Vercel-specific frontend flag。
- 保留 stored settings/processor 数据，只隐藏不可用能力的新增入口和运行配置。

- [ ] **步骤 1：加入 capability=false 回归测试**

测试至少证明：

```ts
it("hides the probe processor option when probe is unavailable")
it("hides probe defaults and scheduled refresh when capabilities are unavailable")
it("does not request or poll scheduled refresh status when scheduler is unavailable")
```

断言：

- 新增 processor 的 options 中没有“测活”；
- Runtime settings 中没有测活 accordion/fields；
- 页面没有定时刷新 section；
- `getScheduledRefreshStatus` 在初始 render 和推进 30 秒后都未调用。

已有 stored probe processor 不得在 capability false 时被自动过滤出 serialized value；如 UI 仍显示该已存储项供用户删除，这是保留数据的兼容行为，不算重新开放新增能力。

- [ ] **步骤 2：运行聚焦 Web 测试**

```bash
pnpm --dir web test:run \
  app/features/subscriptions/components/processor-builder.test.tsx \
  app/features/settings/pages/settings-detail-pages.test.tsx \
  app/features/settings/data/settings-data-hooks.test.tsx
```

预期：若现有 capability gating 完整则直接 PASS；若失败，只在上述最邻近 owner 文件中做最小修复。

- [ ] **步骤 3：运行 Web 静态门禁**

```bash
pnpm --dir web typecheck
pnpm --dir web lint
git diff --check
```

预期：全部 PASS。

- [ ] **步骤 4：按授权决定是否提交**

如已授权：

```bash
git add \
  web/app/features/subscriptions/components/processor-builder.test.tsx \
  web/app/features/settings/pages/settings-detail-pages.test.tsx \
  web/app/features/settings/data/settings-data-hooks.test.tsx
git add -u web/app/features
git commit -m "test(web): hide unavailable runtime capabilities"
```

---

### 任务 8：增加真实 S3-compatible opt-in 集成测试

**文件：**

- 新建：`internal/store/s3_integration_test.go`
- 新建：`internal/app/s3_integration_test.go`
- 修改：`.gitignore`（仅在测试产生新的本地输出时）

**接口：**

- 只在 `SANDRONE_TEST_S3=1` 时连接外部 endpoint。
- 使用独立的 `SANDRONE_TEST_S3_*` 变量，禁止复用生产变量或提交 credentials。

- [ ] **步骤 1：增加显式 opt-in 门槛**

集成测试读取：

```text
SANDRONE_TEST_S3=1
SANDRONE_TEST_S3_ENDPOINT
SANDRONE_TEST_S3_REGION
SANDRONE_TEST_S3_BUCKET
SANDRONE_TEST_S3_PREFIX
SANDRONE_TEST_S3_FORCE_PATH_STYLE
SANDRONE_TEST_S3_ACCESS_KEY_ID
SANDRONE_TEST_S3_SECRET_ACCESS_KEY
SANDRONE_TEST_S3_SESSION_TOKEN
```

未显式设置 `SANDRONE_TEST_S3=1` 时 `t.Skip`；启用后任一必需字段缺失必须失败，不得静默跳过。prefix 必须在测试开始时附加随机 run namespace，并在 `t.Cleanup` 中删除该 namespace 下所有对象。

- [ ] **步骤 2：在真实 endpoint 运行 Store contract**

`TestS3IntegrationStoreContract` 调用任务 2 的共享 contract，并额外覆盖：

- 多页 List；
- overwrite 后立即 Read/Stat/List；
- Delete 后立即 Read/List；
- `settings.json` 的 `WriteAtomic`；
- 并发单 key 写的 last-completing-writer-wins。

- [ ] **步骤 3：运行 app 核心持久化流程**

`internal/app/s3_integration_test.go` 使用 `package app_test` 构造真实 S3 runtime，覆盖：

1. 写入并重新构造 runtime 后读取 settings；
2. subscription/file CRUD；
3. 创建并读取公开 share；
4. 写入 cache 后重建 runtime仍可命中；
5. 导出 backup，清空 namespace，再 restore 并验证资源；
6. backup 不包含 cache key。

所有 fixture 使用 `example.com` 和伪凭据内容，不包含真实订阅或节点。

- [ ] **步骤 4：先在本地兼容服务运行，再以 R2 作为最终证明**

未设置 opt-in 时：

```bash
go test ./internal/store ./internal/app -run S3Integration -count=1
```

预期：测试显示 SKIP，普通 CI 无网络依赖。

有授权且已准备测试 bucket 时：

```bash
SANDRONE_TEST_S3=1 go test \
  ./internal/store ./internal/app \
  -run S3Integration \
  -count=1 \
  -timeout=5m
```

预期：对 Cloudflare R2 全部 PASS，测试 namespace cleanup 后为空。

- [ ] **步骤 5：按授权决定是否提交**

如已授权：

```bash
git add internal/store/s3_integration_test.go internal/app/s3_integration_test.go .gitignore
git commit -m "test(store): verify S3-compatible persistence"
```

---

### 任务 9：补齐 Docker、架构、CLI 与 Vercel + R2 部署文档

**文件：**

- 修改：`docker-compose.yaml`
- 修改：`docs/architecture/storage.md`
- 修改：`docs/reference/cli.md`
- 新建：`docs/how-to/deploy-vercel-r2.md`
- 修改：`docs/README.md`
- 视实际入口变化检查：`README.md`

- [ ] **步骤 1：让 Docker 显式透传通用 S3 环境变量**

在 compose 中保留 filesystem 默认值和 volume，新增：

```yaml
SANDRONE_STORAGE_BACKEND: ${SANDRONE_STORAGE_BACKEND:-filesystem}
SANDRONE_S3_ENDPOINT: ${SANDRONE_S3_ENDPOINT:-}
SANDRONE_S3_REGION: ${SANDRONE_S3_REGION:-}
SANDRONE_S3_BUCKET: ${SANDRONE_S3_BUCKET:-}
SANDRONE_S3_PREFIX: ${SANDRONE_S3_PREFIX:-sandrone/}
SANDRONE_S3_FORCE_PATH_STYLE: ${SANDRONE_S3_FORCE_PATH_STYLE:-false}
SANDRONE_S3_ACCESS_KEY_ID: ${SANDRONE_S3_ACCESS_KEY_ID:-}
SANDRONE_S3_SECRET_ACCESS_KEY: ${SANDRONE_S3_SECRET_ACCESS_KEY:-}
SANDRONE_S3_SESSION_TOKEN: ${SANDRONE_S3_SESSION_TOKEN:-}
```

不得把示例 secret 写成真实值；filesystem quick start 继续开箱即用。

- [ ] **步骤 2：更新 canonical 存储架构**

在 `docs/architecture/storage.md` 正向说明：

- FSStore 与 S3Store 的后端选择；
- S3 的五个必要操作和强一致性要求；
- namespace、虚拟目录、List 分页、Delete not-found、single-PUT `WriteAtomic`；
- credentials、错误脱敏和显式凭据边界；
- Coordinator 仍只提供进程内协调；
- 多实例 restore 需要维护窗口；
- Store-backed cache 的对象请求成本。

删除“仓库只内建 FSStore”的旧事实，不保留迁移墓碑。

- [ ] **步骤 3：更新 CLI 参考**

在 `docs/reference/cli.md` 增加 storage 环境变量表，明确：

- backend 默认 filesystem；
- `--data-dir` 只对 filesystem 生效；
- S3 没有 flags，也不进入 settings/backup；
- R2 region 为 `auto`，常规 R2 `force_path_style=false`；
- `doctor` 在 S3 下会创建并删除 `_doctor/` 临时对象；
- 长驻 S3 `serve` 仍启用 scheduler/probe。

- [ ] **步骤 4：编写 Vercel + R2 操作指南**

`docs/how-to/deploy-vercel-r2.md` 必须包含：

1. 创建私有 R2 bucket；
2. 创建仅限该 bucket object read/write 的 API token；
3. 配置通用 S3 环境变量，endpoint 示例为 `https://<account-id>.r2.cloudflarestorage.com`、region 为 `auto`；
4. 配置非空 `SANDRONE_TOKEN`；
5. Preview 与 Production 使用不同 bucket 或 prefix；
6. Vercel profile 隐藏并拒绝 scheduler/probe；
7. 4.5 MB 请求/响应上限；
8. filesystem backup 到 S3 restore 的维护窗口流程；
9. Preview smoke check；
10. 回滚上一 Vercel deployment，并保持 R2 namespace 不变的注意事项。

指南不得复制完整 HTTP/MCP 契约，只链接 canonical reference。

- [ ] **步骤 5：更新文档索引并检查链接**

在 `docs/README.md` 的“完成任务”加入部署指南。只有 README 当前部署入口直接需要时才增加一条链接，不复制环境变量表。

运行：

```bash
rg -n "只内建.*FSStore|SANDRONE_STORAGE_BACKEND|deploy-vercel-r2" \
  README.md docs docker-compose.yaml
git diff --check
```

预期：旧事实消失，环境变量和指南入口可定位，无空白错误。

- [ ] **步骤 6：按授权决定是否提交**

如已授权：

```bash
git add docker-compose.yaml docs README.md
git commit -m "docs: add Vercel and R2 deployment guide"
```

---

### 任务 10：执行完整门禁与受控 Preview 验收

**文件：**

- 验证所有本计划涉及文件。
- 不创建新的长期 plan/spec 墓碑；功能完成并合入前按仓库规则处理实施文档。

- [ ] **步骤 1：格式化并运行 Go 窄测**

```bash
go fmt ./internal/store ./internal/app ./internal/probe ./internal/service ./internal/entry/cli ./api
go test \
  ./internal/store \
  ./internal/app \
  ./internal/probe \
  ./internal/service \
  ./internal/entry/cli \
  ./internal/entry/httpapi \
  ./internal/entry/mcpapi \
  ./api \
  -count=1
```

预期：全部 PASS。

- [ ] **步骤 2：运行完整本地门禁**

```bash
make ci
pnpm --dir web test:run
pnpm --dir web typecheck
pnpm --dir web lint
pnpm --dir web test:e2e
git diff --check
```

预期：全部成功。`make ci` 覆盖 Go fmt/vet/test/build/lint；Web 四项门禁单独执行。

- [ ] **步骤 3：检查旧装配和敏感信息**

```bash
rg -n "NewWithFS|Runtime\.Engine|config\.LoadDefaultConfig|afero-s3|D1|Cloudflare Worker gateway" \
  internal api docs
rg -n "SANDRONE_S3_(ACCESS_KEY_ID|SECRET_ACCESS_KEY|SESSION_TOKEN)=" \
  --glob '!docs/how-to/deploy-vercel-r2.md' \
  --glob '!docs/superpowers/**' .
git status --short
```

预期：

- 生产 CLI/runtime 不再硬编码 `NewWithFS`；
- 不存在默认 AWS credential chain 或已排除的架构；
- 没有提交实际 S3 credential 值；
- 工作树只包含本任务预期文件和用户原有改动。

- [ ] **步骤 4：在执行外部部署前暂停并取得明确授权**

Vercel Preview、R2 bucket/token 配置会改变外部状态。除非用户在执行阶段明确授权，否则到此停止，把本地门禁和尚未执行的 Preview 验收项交给用户。

- [ ] **步骤 5：经授权后部署隔离的 Preview**

为 Preview 使用独立 bucket 或唯一 prefix，设置文档中的全部环境变量。部署命令与账号选择以当前 Vercel 项目为准；不要在终端输出 token 或 S3 secret。

验收矩阵：

- `GET /version` 与 Web 静态资源；
- bearer token 缺失/错误被拒绝；
- subscription、file、settings CRUD；
- 公开 share；
- conversion；
- MCP discover、资源读取和 management flow；
- `/v1/capabilities/ui` 四项 serverless capability 为 false；
- `/v1/probe`、MCP probe、probe processor 返回 `probe_backend_unavailable`；
- 小于 4.5 MB 的 backup export/restore round trip；
- 触发新 deployment/cold start 后 settings、resource、share 仍存在；
- R2 namespace 中没有 `_doctor/` 残留。

- [ ] **步骤 6：记录最终证据并按授权提交**

记录具体运行命令、通过的 gate 和 Preview deployment URL；不得记录 credentials。如用户已授权 commit，最后检查：

```bash
git diff --cached --check
git status --short
```

使用与实际改动匹配的 Conventional Commit；没有明确授权时不提交、不 push、不创建 PR。

---

## 完成判定

只有同时满足以下条件才算完成：

- filesystem 默认路径、长驻 scheduler 和现有 probe 行为无回归；
- S3Store 通过共享 Store contract、mock 边界测试和真实 R2 opt-in integration；
- CLI、Docker 与 `serve` 能仅通过环境变量选择 S3；
- Vercel 只创建一个 Go Function，复用现有 HTTP/MCP/Web handler；
- Vercel 强制 S3 + token，且 scheduler/probe 后端和 Web 入口均不可用；
- `/v1/probe`、MCP probe、processor/script probe 明确返回 `probe_backend_unavailable`；
- 本地 Go/Web 全门禁通过；
- 经授权的 Preview Deployment 通过 R2 跨 cold-start 持久化和小型 backup round trip；
- 仓库不包含真实凭据、私有数据、生成的 Web 产物或与本任务无关的改动。
