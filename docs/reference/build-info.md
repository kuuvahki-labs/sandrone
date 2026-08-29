# 构建身份

Sandrone 将发布版本、源码 revision 与产物构建时间作为三个独立事实：

- `version` 是面向兼容性和发布说明的规范版本，例如 `0.1.0`；
- `revision` 是生成当前二进制的完整 Git object ID，用于诊断和追溯；
- `build_time` 是构建入口生成的 RFC3339 UTC 时间，用于区分实际产物。

不要把 revision 拼进 `version`。User-Agent、MCP server version 和备份清单中的
`app_version` 都继续使用纯版本号。项目设置未显式覆盖 User-Agent 时，每次远程
请求都使用当前构建的 `sandrone/<version>`，不会把版本字符串固化进设置文件。

规范发布版本只维护在
[`internal/buildinfo/VERSION`](../../internal/buildinfo/VERSION)。Go 二进制嵌入
该文件，Make 和容器构建也从它读取；Dockerfile 与 Compose 不再复制发布版本号。

## 来源与展示

构建信息按以下优先级解析：

1. Make/CI 构建通过 Go linker 显式注入 version、revision 和 build time；
2. 直接在干净 Git worktree 中执行 `go build` 时，revision 回退到 Go 自动写入的
   `vcs.revision`；
3. 裸 `docker build` 显式注入 `dev` 版本且不提供 revision，不会伪装成可追溯
   的发布构建，但仍记录本次容器内编译的 build time。

Go metadata 的 `vcs.time` 是 commit time，不是 binary build time，不能用于区分
同一提交下的不同构建。直接执行 `go build` 不经过项目构建入口，因此不会注入
`build_time`。

`sandrone --version` 和 Web 关于页使用 12 位短 revision，并显示完整 RFC3339
build time；公开 `GET /version` 接口返回完整 revision 和 `build_time`。例如开发
构建显示 `dev (2026-08-30T03:15:42Z)`，tag 构建显示
`v0.1.12 (0123456789ab; 2026-08-30T03:15:42Z)`。
Go metadata 标记为 `vcs.modified=true`，或者 Make 检测到 staged、unstaged 或
untracked 改动时，构建身份统一为 `dev` 且不报告 revision，但仍报告 build time。

## 本地构建参数

普通 Make 构建默认读取版本文件，并在干净 Git worktree 中自动读取当前 HEAD：

```sh
make build-bin
```

也可以显式覆盖 `VERSION`、`REVISION` 和 `BUILD_TIME`。`VERSION` 只允许 ASCII 字母、数字、
点、加号和连字符；一个开头的 `v` 会在运行时移除。`REVISION` 可以为空，否则
必须是完整的 40 位或 64 位十六进制 Git object ID。revision 为空时，有效版本
始终强制为 `dev`；非 `dev` 版本必须与完整 revision 一起提供。`BUILD_TIME` 默认
是当前 RFC3339 UTC 时间，也可显式传入同格式的值，让同批多架构产物共享一个时间。
三者都经过验证后才进入 linker flags。

构建本地镜像使用统一入口：

```sh
make image
make image SANDRONE_IMAGE=ghcr.io/kuuvahki-labs/sandrone:v0.1.0
```

`make image` 默认生成 `ghcr.io/kuuvahki-labs/sandrone:local`。干净 worktree
会向二进制和 OCI labels 传入规范版本及当前 HEAD；dirty worktree 或无 Git
目录与裸 Docker 构建一样改用 `dev`，不会输出看似正式但不可追溯的版本。每次
`make image` 都把新的 build time 作为 Docker build arg，避免构建层缓存沿用旧时间。
`SANDRONE_IMAGE` 同时是 Make 的构建 tag 和 Compose 的运行镜像覆盖变量。本地
`make image` 始终使用当前 Docker daemon 的单一平台，不创建多架构 manifest。

## GitHub Release

发布可以通过两种方式触发：直接推送与规范版本匹配的 `v<version>` Git tag，或在
`main` 上手动运行 `Create Release`。手动流程从最新稳定 `vMAJOR.MINOR.PATCH` tag
自动递增 patch，更新并提交 `internal/buildinfo/VERSION`，创建 annotated tag，再以
该 tag 显式运行 CI。例如最新稳定 tag 为 `v0.1.0` 时，下一次手动发布会创建
`v0.1.1`。普通分支 push、pull request 和以分支 ref 手动运行的 CI 不会创建
GitHub Release，也不会上传发布附件。

发布版本采用比本地构建身份更严格的规则：只允许 ASCII 字母、数字、点和连字符，
发布版本不允许加号，并且最多 127 个字符。CI 会添加 `v` 前缀，因此完整 OCI tag
最多 128 个字符。这个限制只适用于发布；本地构建身份仍可使用加号表达 build
metadata。

每个 GitHub Release 固定包含以下三个附件：

- `sandrone_linux_amd64.tar.gz`，用于 `linux/amd64`；
- `sandrone_linux_arm64.tar.gz`，用于 `linux/arm64`；
- `checksums.txt`，包含上述两个压缩包的 SHA-256 校验值。

附件名不重复版本号；GitHub Release tag 是下载路径中的版本边界。安装脚本既可以用
`releases/download/v<version>/sandrone_linux_<arch>.tar.gz` 固定版本，也可以用
`releases/latest/download/sandrone_linux_<arch>.tar.gz` 跟随最新稳定版本。

两个压缩包都只包含 `sandrone` 可执行文件和 `LICENSE`。可执行文件已经嵌入同版本
Web UI，并使用与 Docker 镜像一致的默认 build tags，包含 TCP 与 sing-box probe
backend；用户不需要再组合前端或测活附件。下载三个附件到同一目录后执行：

```sh
sha256sum -c checksums.txt
```

本地执行 `make release-artifacts` 会先构建 Web UI，再生成两个自包含二进制归档和
校验文件。

工作树存在未提交改动、只需要生成本地部署快照时，使用：

```sh
make snapshot-artifacts
```

该目标同样构建 `linux/amd64`、`linux/arm64` 和 `checksums.txt`，输出到
`dist/snapshot/`。快照身份固定为 `version=dev` 且不报告 revision；同一次任务的
多架构二进制共享 build time。快照不会覆盖 `dist/` 下的正式发布附件，也不用于
GitHub Release。

纯 `vMAJOR.MINOR.PATCH` tag 创建正式 Release；其他与版本文件匹配的 tag（例如
`v0.1.0-rc.1`）创建 prerelease。重新运行同一个 tag 的发布任务会替换同名附件，
不会创建第二个 Release。

## 容器身份

Docker build context 继续排除 `.git`，避免发送仓库历史、扩大 context 和破坏
缓存。因此直接运行 `docker build` 不会推测 commit。只覆盖非 `dev` 的
`VERSION` 而不同时提供完整 `REVISION` 会直接失败：

```sh
docker build -t sandrone:dev .
docker run --rm sandrone:dev --version
# sandrone version dev (2026-08-30T03:15:42Z)
```

需要本地可追溯镜像时，在干净 worktree 中执行 `make image`。CI 使用 Buildx；
pull request、`main` 和以分支 ref 手动运行的 CI 只验证 `linux/amd64` 容器；
`v<version>` tag 构建并发布 `linux/amd64` 和 `linux/arm64` 的 GHCR manifest。
Docker pull 或 run 会按宿主机架构自动选择对应镜像：

- pull request、`main` 和以分支 ref 手动运行的 CI 通过独立任务验证 `:ci` 镜像，
  并与 Go/Web 检查并行，不推送到 GHCR；
- `v<version>` Git tag push 或以该 tag ref 手动调度的 CI 才推送双架构镜像，并保留
  `v` 的同名镜像 tag，例如 `ghcr.io/kuuvahki-labs/sandrone:v0.1.0`；
- 只有版本顺序最高、格式为 `vMAJOR.MINOR.PATCH` 的稳定版本才同时更新 `latest`；
- 预发布 tag 只发布自己的同名 tag；
- CI 不创建或发布 `sha-*` 镜像 tag，需要精确复现时使用镜像 digest。

Dockerfile 的 Web 和 Go builder 固定运行在 `$BUILDPLATFORM`。Web 资产只在 runner
原生架构构建；最终 Go 二进制通过目标 `GOOS`/`GOARCH` 交叉编译，因此发布 ARM64
镜像时不会用 QEMU 执行 pnpm 或 Go 编译。最终 Debian runtime 层仍按目标平台组装。
BuildKit 使用命名的 GitHub Actions 缓存复用依赖和编译层；缓存未命中只会增加
构建时间，不改变镜像内容或发布规则。

普通 CI 的完整容器构建同时验证 Web 资产生成、Go embed 和最终二进制，因此不再
单独重复运行嵌入式 Web UI 构建任务。tag 的容器发布与 GitHub Release 均须等待
Go/Web 检查通过，随后并行发布；`make release-artifacts` 在编译原生二进制前构建
一次 Web 资产。容器和原生包都从二进制内的同一嵌入式文件系统提供 Web UI，容器
运行层不再复制第二份 `/app/static`。

容器发布与 GitHub Release 分别使用自己的 FIFO 队列串行执行，最多各保留 100 个
pending 任务；同一个 tag 的两类发布仍可并行。容器发布前会重新获取远端 tags；
旧版本 tag 即使晚创建或晚完成，也只会写入自己的版本 tag，不会覆盖较新正式版本
的 `latest`。

仓库的 Compose 配置默认运行 `ghcr.io/kuuvahki-labs/sandrone:latest`，不会构建
当前 worktree。本地镜像需要显式覆盖：

```sh
make image
SANDRONE_IMAGE=ghcr.io/kuuvahki-labs/sandrone:local docker compose up
```

最终镜像还包含标准 OCI labels：

- `org.opencontainers.image.version`；
- `org.opencontainers.image.revision`；
- `org.opencontainers.image.source`。

镜像被复制或重新打 tag 后，OCI revision label 与二进制 `/version` 仍可用于确认
实际来源。
