# 构建身份

Sandrone 将发布版本与源码 revision 作为两个独立事实：

- `version` 是面向兼容性和发布说明的规范版本，例如 `0.1.0`；
- `revision` 是生成当前二进制的完整 Git object ID，用于诊断和追溯。

不要把 revision 拼进 `version`。User-Agent、MCP server version 和备份清单中的
`app_version` 都继续使用纯版本号。

规范发布版本只维护在
[`internal/buildinfo/VERSION`](../../internal/buildinfo/VERSION)。Go 二进制嵌入
该文件，Make 和容器构建也从它读取；Dockerfile 与 Compose 不再复制发布版本号。

## 来源与展示

构建信息按以下优先级解析：

1. Make/CI 发布构建通过 Go linker 显式注入；
2. 直接在干净 Git worktree 中执行 `go build` 时，revision 回退到 Go 自动写入的
   `vcs.revision`；
3. 裸 `docker build` 显式注入 `dev` 版本且不提供 revision，不会伪装成可追溯
   的发布构建。

`sandrone --version` 和 Web 关于页使用 12 位短 revision，便于阅读；公开
`GET /version` 接口返回完整 revision。revision 未知时只展示版本号。
Go metadata 标记为 `vcs.modified=true`，或者 Make 检测到 staged、unstaged 或
untracked 改动时，构建身份统一为 `dev` 且不报告 revision。

## 本地构建参数

普通 Make 构建默认读取版本文件，并在干净 Git worktree 中自动读取当前 HEAD：

```sh
make build-bin
```

也可以显式覆盖 `VERSION` 和 `REVISION`。`VERSION` 只允许 ASCII 字母、数字、
点、加号和连字符；一个开头的 `v` 会在运行时移除。`REVISION` 可以为空，否则
必须是完整的 40 位或 64 位十六进制 Git object ID。revision 为空时，有效版本
始终强制为 `dev`；非 `dev` 版本必须与完整 revision 一起提供。两者都经过验证
后才进入 linker flags。

构建本地镜像使用统一入口：

```sh
make image
make image SANDRONE_IMAGE=ghcr.io/kuuvahki-labs/sandrone:v0.1.0
```

`make image` 默认生成 `ghcr.io/kuuvahki-labs/sandrone:local`。干净 worktree
会向二进制和 OCI labels 传入规范版本及当前 HEAD；dirty worktree 或无 Git
目录与裸 Docker 构建一样改用 `dev`，不会输出看似正式但不可追溯的版本。
`SANDRONE_IMAGE` 同时是 Make 的构建 tag 和 Compose 的运行镜像覆盖变量。本地
`make image` 始终使用当前 Docker daemon 的单一平台，不创建多架构 manifest。

## GitHub Release

只有推送与规范版本匹配的 `v<version>` Git tag 才会发布。例如
`internal/buildinfo/VERSION` 为 `0.1.0` 时，发布 tag 必须是 `v0.1.0`。分支 push、
pull request 和手动 CI 都不会创建 GitHub Release，也不会上传发布附件。

发布版本采用比本地构建身份更严格的规则：只允许 ASCII 字母、数字、点和连字符，
发布版本不允许加号，并且最多 127 个字符。CI 会添加 `v` 前缀，因此完整 OCI tag
最多 128 个字符。这个限制只适用于发布；本地构建身份仍可使用加号表达 build
metadata。

每个 GitHub Release 固定包含以下三个附件：

- `sandrone_<version>_linux_amd64.tar.gz`，用于 `linux/amd64`；
- `sandrone_<version>_linux_arm64.tar.gz`，用于 `linux/arm64`；
- `checksums.txt`，包含上述两个压缩包的 SHA-256 校验值。

两个压缩包都只包含 `sandrone` 可执行文件和 `LICENSE`。下载三个附件到同一目录
后执行：

```sh
sha256sum -c checksums.txt
```

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
# sandrone version dev
```

需要本地可追溯镜像时，在干净 worktree 中执行 `make image`。CI 使用 Buildx；
pull request、`main` 和手动 CI 只验证 `linux/amd64` 容器；`v<version>` tag 构建并发布
`linux/amd64` 和 `linux/arm64` 的 GHCR manifest。Docker pull 或 run 会按宿主机架构
自动选择对应镜像：

- pull request、`main` 和手动 CI 通过独立任务验证 `:ci` 镜像，并与 Go/Web
  检查并行，不推送到 GHCR；
- 只有 `v<version>` Git tag push 才推送双架构镜像，并保留 `v` 的同名镜像 tag，
  例如 `ghcr.io/kuuvahki-labs/sandrone:v0.1.0`；
- 只有版本顺序最高、格式为 `vMAJOR.MINOR.PATCH` 的稳定版本才同时更新 `latest`；
- 预发布 tag 只发布自己的同名 tag；
- CI 不创建或发布 `sha-*` 镜像 tag，需要精确复现时使用镜像 digest。

Dockerfile 的 Web 和 Go builder 固定运行在 `$BUILDPLATFORM`。Web 资产只在 runner
原生架构构建；最终 Go 二进制通过目标 `GOOS`/`GOARCH` 交叉编译，因此发布 ARM64
镜像时不会用 QEMU 执行 pnpm 或 Go 编译。最终 Debian runtime 层仍按目标平台组装。
BuildKit 使用命名的 GitHub Actions 缓存复用依赖和编译层；缓存未命中只会增加
构建时间，不改变镜像内容或发布规则。

普通 CI 的完整容器构建同时验证 Web 资产生成、Go embed 和最终二进制，因此不再
单独重复运行嵌入式 Web UI 构建任务。tag 发布任务仍须等待 Go/Web 检查通过；
GitHub Release 产物继续独立构建 Web 资产和二进制归档。

容器发布任务使用同一 FIFO 队列串行执行，最多保留 100 个 pending 任务。发布
前会重新获取远端 tags；旧版本 tag 即使晚创建或晚完成，也只会写入自己的版本
tag，不会覆盖较新正式版本的 `latest`。

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
