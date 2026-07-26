# Sandrone

Sandrone 是一个轻量的订阅聚合、节点处理与客户端配置生成工具。它以统一的 `NodeIR` 和 `FileSpec` 连接订阅输入、处理链与不同客户端输出，并提供 CLI、HTTP API、MCP、静态 Web UI 和可嵌入 Go API。

许可证：[AGPL-3.0-or-later](LICENSE)。

## 快速运行

安装 Docker 后，在仓库根目录运行：

```sh
docker compose up --pull always
```

服务随后可从 `http://127.0.0.1:1137` 访问。Compose 提供仅供本地开发的 bearer token `sandrone`，并把数据保存在 `sandrone-data` volume；对外提供服务前必须通过 `SANDRONE_TOKEN` 更换 token。

可以用公开版本接口确认服务已启动：

```sh
curl http://127.0.0.1:1137/version
```

## 从这里继续

- 第一次在本地转换节点：阅读[第一次转换](docs/tutorials/first-conversion.md)。
- 第一次通过图形界面管理订阅并生成配置：阅读[第一次使用 Web UI](docs/tutorials/first-web-ui.md)。
- 查找 CLI 命令与输出约定：阅读 [CLI 参考](docs/reference/cli.md)。
- 管理 HTTP 资源与服务：从 [HTTP API 参考](docs/reference/http-api/README.md)进入。
- 接入 MCP 客户端：阅读 [MCP 参考](docs/reference/mcp.md)。
- 生成 Mihomo、sing-box 或 Shadowrocket 配置：阅读[渲染客户端配置](docs/how-to/render-client-config.md)和 [FileSpec 参考](docs/reference/file-spec.md)。
- 编写节点或文件处理脚本：阅读[编写 processor 脚本](docs/how-to/write-processor-script.md)。
- 理解分层、领域模型与数据流：阅读[架构总览](docs/architecture/overview.md)。
- 在 Go 程序中嵌入转换核心：从 [`pkg/sandrone`](pkg/sandrone) 开始。
- 开发静态 Web UI：阅读 [Web 开发说明](web/README.md)。
- 查找全部教程、操作指南、参考和架构文档：打开[文档索引](docs/README.md)。
- 参与开发：遵循[贡献指南](CONTRIBUTING.md)。

## 隐私与安全

不要在 issue、PR、日志、fixture 或文档示例中提交真实订阅链接、节点 URI、token、cookie、私钥、真实服务地址或未脱敏配置。安全漏洞请按[安全策略](SECURITY.md)使用 GitHub private vulnerability reporting 报告，不要公开披露细节。

## 致谢

- [MetaCubeX/mihomo](https://github.com/MetaCubeX/mihomo)
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box)
- [LOWERTOP/Shadowrocket](https://github.com/LOWERTOP/Shadowrocket)
- [blackmatrix7/ios_rule_script](https://github.com/blackmatrix7/ios_rule_script)
- [tindy2013/subconverter](https://github.com/tindy2013/subconverter)
- [sub-store-org/Sub-Store](https://github.com/sub-store-org/Sub-Store)
- [shadowsocks/shadowsocks-org](https://github.com/shadowsocks/shadowsocks-org)
- [v2fly/v2fly-github-io](https://github.com/v2fly/v2fly-github-io)
- [XTLS/Xray-core](https://github.com/XTLS/Xray-core)
- [trojan-gfw/trojan](https://github.com/trojan-gfw/trojan)
- [apernet/hysteria](https://github.com/apernet/hysteria)
- [tuic-protocol/tuic](https://github.com/tuic-protocol/tuic)
- [WireGuard/wireguard-go](https://github.com/WireGuard/wireguard-go)
