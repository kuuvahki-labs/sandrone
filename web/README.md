# Sandrone Web UI

Sandrone Web UI 是一个静态 React Router SPA。本页提供安装、开发、运行、
构建和验证的快速入口；模块所有权、依赖与测试放置规则见
[Web UI 模块约定](AGENTS.md)。

以下 pnpm 命令都从本 `web/` 目录运行；`make` 和 `go` 等仓库级命令会另行
注明从仓库根目录运行。

## 环境与安装

Web 开发使用 Node.js 24.17.0 LTS（见 [`.nvmrc`](.nvmrc)）和 pnpm
11.24.0：

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

产物位于 `web/build/client`。要让 Go server 嵌入 Web UI，在仓库根目录执行：

```sh
make build-webui
```

该目标安装锁定依赖、构建并复制资源到 `internal/entry/webui/static`，供随后
Go 构建嵌入。两处产物都不提交；生产二进制不需要 Node server，也不读取外部
静态目录。

已有测试通过的产物时，可在仓库根目录执行
`WEBUI_PREBUILT_DIR="$PWD/web/build/client" make build-webui`，直接复制完整目录
（包括预压缩资源），跳过依赖安装和重新构建。目录必须包含非空 `index.html`。
CI 中 Vercel 和 Release 使用同一次 workflow 中通过 E2E 的 Web 产物。

## 运行

本地预览已经构建的 client：

```sh
pnpm start
```

预览地址默认为 `http://127.0.0.1:4173`。完成 `make build-webui` 后，在仓库根目录
启动带嵌入资源的 Sandrone HTTP server：

```sh
go run -mod=readonly -tags probe_singbox ./cmd/sandrone serve
```

然后访问 `http://127.0.0.1:1137/`。

列表的短期复用、后台刷新与写后失效见[页面数据复用](AGENTS.md#页面数据复用)。

程序日志的页面使用与保留范围见[程序日志](../docs/reference/http-api/logs.md)。

## 验证

验证范围统一按[贡献指南](../CONTRIBUTING.md#选择验证范围)选择：局部行为先运行
相关 Vitest；共享逻辑变化扩大测试范围，路由、浏览器集成和响应式变化使用 Playwright。
默认 Playwright 流程已构建 SPA，无需在同一版本上重复执行 build。
