# 贡献指南

本文件是 Sandrone 人类贡献流程的唯一权威。自动化代理的快速约束见 [AGENTS.md](AGENTS.md)，产品与系统文档从[文档索引](docs/README.md)进入。

参与讨论和提交变更即表示你同意遵守[行为准则](CODE_OF_CONDUCT.md)。

## 准备开发环境

在仓库根目录运行 Go 命令。建议准备：

- Go `1.27.0`，版本以 `go.mod` 为准。
- Node.js `24.17.0` 和 pnpm `11.24.0`，仅 Web 开发需要。
- Docker 与 Docker Compose，用于验证容器运行路径。
- 支持当前 Go 版本的 `golangci-lint`，仅 `make ci` 或 `make lint` 需要。

首次进行 Web 开发时安装依赖：

```sh
nvm install 24.17.0
nvm use 24.17.0
corepack enable
corepack install --global pnpm@11.24.0
pnpm --dir web install --frozen-lockfile
```

运行 `make help` 可以查看维护的 Make targets。`make build` 和 `make build-bin` 会联网生成规则集目录快照；默认质量门 `make check` 使用已有源码完成检查，不依赖该下载步骤。发布构建的 `VERSION`、`REVISION` 和容器 metadata 契约见[构建身份](docs/reference/build-info.md)。

## 贡献流程

1. 先确认改动要解决的用户问题、公开契约和兼容性影响。
2. 阅读相关架构页、reference 和邻近代码，沿用现有分层与命名。
3. 在最接近行为契约的层级补充或更新测试，再实现最小完整改动。
4. 保持入口层只做协议适配，把业务编排留在 service。
5. 先运行目标明确的窄测试，再运行与改动范围匹配的完整质量门。
6. 同步更新拥有该事实的 canonical 文档，其他页面只增加必要链接。
7. 删除已被取代的代码、测试、fixture、示例和说明，并扫描旧标识。
8. 提交聚焦的 commit，填写 PR 模板并说明验证、文档和兼容性结论。

不要覆盖或顺手整理与当前贡献无关的工作树改动。需要扩大范围时，先在 issue 或 PR 中说明原因。

## 架构边界

[架构总览](docs/architecture/overview.md)说明层级职责和依赖方向；领域对象见[领域模型](docs/architecture/domain-model.md)。完整数据流分别归属[节点管线](docs/architecture/node-pipeline.md)和[文件管线](docs/architecture/file-pipeline.md)，运行时观测与持久化边界见[节点探测](docs/architecture/probing.md)和[存储架构](docs/architecture/storage.md)。

所有变更都必须保持以下约束：

- `internal/service` 是唯一业务编排层；CLI、HTTP、MCP 和 Web 入口调用 service，不复制业务逻辑。
- `internal/adapter/*` 负责外部格式解析与渲染，不直接读写 store。
- processor 不 import adapter，也不绕过受控 API 访问网络或文件系统。
- service 和 domain 不 import entrypoint framework 类型。
- `FileSpec.kind` 显式使用 canonical 值，包括 `static`；typed 公共 `config` 只含 `subscriptions` 和 JSON object `settings`。
- typed-file driver 严格解码自身 settings；file-stage processor 按声明顺序执行。
- Web 依赖方向、模块所有权和 `FileDriver` 边界以 [Web 模块约定](web/AGENTS.md)为准。

## 选择验证范围

Go 的默认质量门是：

```sh
make check
```

它运行格式检查、`go vet`、默认 Go 测试和 CLI 构建检查。CI 的完整门禁是：

```sh
make ci
```

Makefile 默认使用
`GOFLAGS=-mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls`。
运行窄测试时也应保留这些 flag：

```sh
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls ./internal/service
go test -mod=readonly -tags probe_singbox,with_quic,with_wireguard,with_utls -run '^TestName$' ./internal/service
make test PKGS=./internal/service TESTFLAGS='-run ^TestName$'
```

按变更类型选择额外覆盖：

- adapter：修改任一格式的 parser 或 renderer 时，不得只按当前格式局部推导。
  先确认来源字段与 canonical `NodeIR` 字段的语义是否真正等价，再沿
  “来源格式 → `NodeIR` → 所有目标格式”检查全部受影响路径，包括其它 parser、
  renderer、`nodevalidation`、capability catalog，以及 raw/lossy/skip warning。
  字段同名不代表语义相同；共享 IR 语义发生变化时，除当前 adapter 的
  parse/render 单元测试、能力断言和必要的 golden fixture 外，至少补一条跨格式
  转换测试。新增 `NodeIR` 字段或扩大既有字段值域前，必须按
  [字段接纳与 warning 处置](docs/architecture/node-pipeline.md#字段接纳与-warning-处置)
  记录来源证据、协议语义与实现私有配置的边界、未知值策略以及所有目标的
  supported/lossy/skip 结论。warning 本身不构成兼容需求，不允许用 probe 或
  单一 renderer 特判绕过该流程。
- processor、service、store、entrypoint：在最接近公开或层间契约的位置测试；文件流不要只依赖 renderer golden。
- probe：默认门禁覆盖 sing-box；修改 Mihomo backend 时额外运行
  `go test -mod=readonly -tags probe_mihomo ./internal/probe ./internal/service`。
- 用户可见 API、CLI、文件模型或 processor 行为：同时核对 canonical reference 和相关 tutorial/how-to。
- 删除功能：用 `rg` 扫描旧标识，除明确的兼容说明外应为零命中。

### 跨协议与客户端影响矩阵

“全局考虑”要求审查完整影响面，不要求无差别修改所有实现。每个必查项都必须得出
`已修改`、`已有测试证明无需修改` 或 `不适用（附原因）` 之一；不能因为当前 fixture、
目标客户端或报错路径只出现一个协议，就省略其它相关项。

最小审查范围按改动性质确定：协议 canonical 语义变化覆盖该协议的全部输入和客户端
输出；客户端共享 adapter 变化覆盖该客户端支持的全部协议；domain、service
normalization/validation 或 shared helper 变化覆盖全部调用者、受影响协议和客户端。

| 修改触点 | 必须审查的代码与契约 |
| --- | --- |
| `internal/domain/node*.go`、协议 option、枚举或 canonical 常量 | 所有 parser/renderer、`nodevalidation`、capability catalog、JSON Nodes、script envelope、`pkg/sandrone` 公共别名，以及节点 clone、preview identity、cache/hash/比较逻辑 |
| `internal/adapter/shared` 的 helper、字段表或 source ref | 用 `rg` 找出全部调用者；检查每个调用协议、输入格式和目标客户端，不能只测新增分支 |
| 任一 parser 或来源别名/默认值 | 同协议的其它 parser、全部 renderer、Raw/unknown warning、validation、capability parse 声明和至少一条跨格式转换 |
| 任一 renderer 或客户端字段映射 | 该客户端支持的全部协议分支、skip/lossy warning、capability render 声明和代表性跨协议测试；若改变 canonical 解释，同时检查其它 renderer |
| `internal/service` 的节点 normalize/validate/输入编排，或 nodes-stage processor/script 节点结构 | 显式/自动/remote/local/ref/inline 输入、processor 前后 validation、直接 render、subscription/file flow、script envelope/schema 和 probe 前校验 |
| capability catalog、warning/error code、report 聚合或上游 revision | 对应 parser/renderer、supported/lossy/raw_only 互斥关系、source ref、阶段与顶层 report、HTTP/CLI/MCP 展示、聚合脱敏和测试计数 |
| probe payload、core backend 或探测前 renderer | sing-box 与 Mihomo 等已注册 core、节点级隔离、raw CLI 路径与保存订阅 processor 链；probe 不得补做 canonical 修复 |
| 删除协议、字段、客户端能力或兼容分支 | parser、renderer、validation、capability、processor/script/API、fixture、文档和旧标识全仓扫描 |

审查时先用 `rg` 确认定义、读写点和 switch/capability 分支，再选择测试。共享 IR 或
共享语义变化至少需要一条“一个来源 → `NodeIR` → 两个语义不同的目标”跨格式测试；
客户端共享代码变化至少覆盖两个受影响协议。若实际只支持一个目标或协议，应在 PR
中明确写出该事实，而不是省略影响分析。

Web 改动按风险运行：

```sh
pnpm --dir web typecheck
pnpm --dir web lint
pnpm --dir web test:run
pnpm --dir web test:e2e
```

组件和交互行为使用 Vitest/Testing Library；路由级 smoke、浏览器集成或响应式行为使用 Playwright。构建最终静态资源时运行 `make build-webui`。

文档改动至少运行 `git diff --check`，并检查相对链接、片段锚点、示例脱敏和页面职责。提交 PR 前应运行 `make check`；若环境阻止某项验证，请在 PR 中写明命令、阻碍和已完成的替代检查。

## 文档政策

公开文档只描述当前产品，并按读者意图区分：

- Tutorial 带第一次使用者完成一条有可观察结果的路径，目标约 `120–200` 行。
- How-to 假设读者已有基础，只解决一个任务，目标约 `60–150` 行。
- Reference 可检索、穷举当前契约，通常约 `80–250` 行。
- Architecture 解释边界、关系和数据流，通常约 `100–180` 行，复杂页面不超过约 `250` 行。

入口页同样有预算：根 README 约 `60–100` 行，Docs 索引约 `30–50` 行，本文约 `120–180` 行，AGENTS 约 `50–80` 行。预算用于发现职责膨胀，不应通过空话凑行数。

同一事实只在一个位置完整说明：

- CLI 契约归属 [CLI 参考](docs/reference/cli.md)。
- HTTP 通用约定和资源接口归属 [HTTP API 参考](docs/reference/http-api/README.md)及其专题页。
- MCP transport、tools、resources、prompts 和输出边界归属 [MCP 参考](docs/reference/mcp.md)。
- 文件字段、processor、脚本 API 分别归属 [FileSpec](docs/reference/file-spec.md)、[Processors](docs/reference/processors.md)和[脚本 API](docs/reference/scripting-api.md)。
- 格式能力、错误诊断分别归属[能力参考](docs/reference/capabilities.md)和[错误参考](docs/reference/errors.md)。
- 完整 nodes flow 和 file flow 只在各自架构页出现；README、Docs 索引和架构总览只做摘要与导航。

历史名称只应出现在仍有效且集中的安全、协议、迁移或数据保护说明中。普通 fixture 和示例使用当前规范结构。

实施期间可以临时维护 spec 或 plan，但完成交付前必须删除；长期公开文档不保存 agent 执行清单、实施日志或报告。删除功能时也要删除只用于描述或证明旧行为的测试、fixture、文档和临时规划材料，历史由 Git 保存。

示例只能使用 `example.com`、文档保留地址和占位凭据。不得提交本机路径、私有 fixture、真实订阅、节点 URI、token、cookie、私钥或运行时数据。

## Commit 与 Pull Request

提交信息使用项目现有的 Conventional Commit 前缀，例如 `feat:`、`fix:`、`refactor:`、`docs:` 和 `chore:`；有明确子域时可使用 scope。每个 commit 应只有一个可独立审阅的关注点。

PR 描述应包含行为变化、相关 issue、验证命令、文档更新和兼容性影响。可见 Web UI 变化请附已脱敏的截图或录屏，并使用仓库的 [PR 模板](.github/pull_request_template.md)。

提交前确认：

- [ ] 改动遵守架构与依赖边界。
- [ ] adapter 变更已评估所有输入、输出、IR 校验、能力声明和跨格式影响。
- [ ] 新增字段或扩大值域已给出字段接纳证据、未知值策略和私有配置边界；不涉及则已确认。
- [ ] 测试覆盖最接近的行为契约。
- [ ] 已运行 `make check`，或在 PR 中说明未运行原因。
- [ ] Web、probe 或集成改动已运行对应专项检查。
- [ ] 用户可见行为已更新唯一 canonical 文档。
- [ ] 过时实现、测试、fixture、示例和临时 spec/plan 已清理。
- [ ] 已扫描旧标识和仓内链接。
- [ ] 示例、日志、配置、截图和输入均已脱敏。
- [ ] PR 说明兼容性影响以及必要的迁移或回滚方式。

## 安全报告

安全漏洞请按 [SECURITY.md](SECURITY.md) 使用 GitHub private vulnerability reporting 报告。不要在公开 issue 或 PR 中披露漏洞细节、真实凭据或未脱敏诊断信息。
