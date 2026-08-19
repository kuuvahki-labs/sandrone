# Sandrone Web UI

Sandrone Web UI 是一个静态 React Router SPA。本页提供安装、开发、运行、
构建和验证的快速入口；模块所有权、依赖与测试放置规则见
[Web UI 模块约定](AGENTS.md)。

以下 pnpm 命令都从本 `web/` 目录运行；`make` 和 `go` 等仓库级命令会另行
注明从仓库根目录运行。

## 环境与安装

Web 开发使用 Node.js 24.17.0 LTS（见 [`.nvmrc`](.nvmrc)）和 pnpm
11.5.2：

```sh
corepack enable
corepack install
pnpm install --frozen-lockfile
```

## 开发

前端与后端使用两个独立开发进程。先在仓库根目录启动 Go 后端；这个命令不要求先
生成或嵌入 Web 资源：

```sh
go run -mod=readonly -tags probe_singbox ./cmd/sandrone serve
```

然后在 `web/` 目录启动 React Router/Vite 开发服务器：

```sh
pnpm dev
```

页面默认位于 `http://localhost:5173`。开发服务器把 `/healthz`、
`/version`、`/convert`、`/v1/*` 和 `/s/*` 请求代理到
`http://127.0.0.1:1137`；可按需覆盖后端地址：

```sh
SANDRONE_DEV_API_TARGET=http://127.0.0.1:18080 pnpm dev
```

## 构建

只生成 Web 产物：

```sh
pnpm build
```

Web 构建只生成 `web/build/client` 中的 client-side 静态资源，并为不小于
8 KiB 的 JavaScript 和 CSS 生成构建期 Brotli 副本。普通资源保留为兼容兜底，
生产 Sandrone HTTP server 会按请求的 `Accept-Encoding` 自动选择 `.br` 表示。
如需让仓库内的 Sandrone HTTP server 按默认路径发现这些资源，在仓库根目录执行：

```sh
make build-webui
```

该目标会安装锁定依赖、构建 Web UI，并把 client 产物复制到
`internal/entry/webui/static`，供随后的 Go 构建嵌入二进制。`web/build/` 和复制后
的静态资源均为生成物，不应提交；仓库只保留让无前端产物的普通 Go 构建仍可编译
的占位文件。

## 运行

本地预览已经构建的 client：

```sh
pnpm start
```

预览地址默认为 `http://127.0.0.1:4173`。生产运行时不需要 Node server；在仓库
根目录先执行 `make build-webui`，再启动 Sandrone HTTP server。Go 构建会把当前
静态资源嵌入可执行文件：

```sh
go run -mod=readonly -tags probe_singbox ./cmd/sandrone serve
```

然后访问 `http://127.0.0.1:1137/`。

发布二进制只使用构建时嵌入的 Web 资源，不读取运行时外部静态目录。

## 验证

迭代时先运行最窄的相关 Vitest 文件；交付前运行：

```sh
pnpm test:run
pnpm typecheck
pnpm lint
pnpm build
```

涉及路由流程、响应式布局或准备合并、发版时，运行精简的桌面与移动端
Playwright smoke：

```sh
pnpm test:e2e
```
