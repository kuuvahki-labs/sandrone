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

启动 React Router/Vite 开发服务器：

```sh
pnpm dev
```

页面默认位于 `http://localhost:5173`。开发服务器把 `/healthz`、
`/version`、`/v1/*` 和 `/s/*` 请求代理到
`http://127.0.0.1:1137`；可按需覆盖后端地址：

```sh
SANDRONE_DEV_API_TARGET=http://127.0.0.1:18080 pnpm dev
```

## 构建

只生成 Web 产物：

```sh
pnpm build
```

Web 构建只生成 `web/build/client` 中的 client-side 静态资源。如需让仓库内
的 Sandrone HTTP server 按默认路径发现这些资源，在仓库根目录执行：

```sh
make build-webui
```

该目标会安装锁定依赖、构建 Web UI，并把 client 产物复制到
`internal/entry/webui/static`。`web/build/` 和复制后的静态资源均为生成物，
不应提交。

## 运行

本地预览已经构建的 client：

```sh
pnpm start
```

预览地址默认为 `http://127.0.0.1:4173`。生产运行时不需要 Node server；
在仓库根目录先执行 `make build-webui`，再启动 Sandrone HTTP server：

```sh
go run -mod=readonly -tags probe_singbox ./cmd/sandrone serve http
```

然后访问 `http://127.0.0.1:1137/`。

## 验证

迭代时先运行最窄的相关 Vitest 文件；交付前运行：

```sh
pnpm test:run
pnpm typecheck
pnpm lint
pnpm build
```

涉及路由流程时，本地默认运行核心桌面链路的 Playwright smoke：

```sh
pnpm test:e2e:smoke
```

涉及响应式行为、跨 viewport 风险或准备合并、发版时，再运行完整 E2E：

```sh
pnpm test:e2e
```
