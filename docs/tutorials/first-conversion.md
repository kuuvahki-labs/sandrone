# 第一次转换

本教程带你把一份本地 URI 列表转换成 Mihomo 节点片段。完成后，你会得到一个可检查的 YAML 文件；整个过程不启动服务，也不保存资源。

## 准备

需要：

- Go 1.25.11；
- 一个 Sandrone 仓库 checkout；
- 支持 POSIX shell 的终端。

在仓库根目录构建 CLI：

```sh
make build-bin BIN=sandrone
```

确认二进制可运行：

```sh
./sandrone --version
```

终端应输出 `sandrone version ...`。

## 1. 创建示例输入

创建 `nodes.txt`：

```sh
printf '%s\n' \
  'ss://aes-128-gcm:demo-password@node-a.example.com:8388#Node-A' \
  'ss://aes-128-gcm:demo-password@node-b.example.com:8389#Node-B' \
  > nodes.txt
```

这里使用 `example.com` 和示例凭据，不会连接真实节点。检查文件：

```sh
wc -l nodes.txt
```

预期为两行。

## 2. 执行转换

运行：

```sh
./sandrone convert \
  --from uri-list \
  --to mihomo-proxies \
  --input nodes.txt \
  --output proxies.yaml
```

命令成功时不需要额外交互，结果写入 `proxies.yaml`。

## 3. 检查结果

查看文件：

```sh
sed -n '1,40p' proxies.yaml
```

输出应是包含 `proxies` 的 YAML，并能看到 `Node-A` 和 `Node-B`。Sandrone 在这一步执行：

```text
读取输入 -> 解析 URI -> 规范化节点 -> 渲染 Mihomo 节点
```

它只生成节点片段，不会补齐 DNS、代理组或规则等完整客户端配置。

## 4. 查看诊断报告

如需保存本次转换的 source trace、warning 和计数，再运行：

```sh
./sandrone convert \
  --from uri-list \
  --to mihomo-proxies \
  --input nodes.txt \
  --output proxies.yaml \
  --report-output report.json
```

查看报告摘要：

```sh
sed -n '1,40p' report.json
```

没有 warning 不代表节点可连接；转换只验证输入和目标格式。连通性探测是独立操作。

## 下一步

- 要在图形界面中聚合并保存多个来源，继续阅读[第一次使用 Web UI](first-web-ui.md)。
- 要生成完整 Mihomo、sing-box 或 Shadowrocket 文件，阅读[渲染客户端配置](../how-to/render-client-config.md)。
- 需要其它输入、输出和 flag 时，查阅 [CLI 参考](../reference/cli.md)与[能力参考](../reference/capabilities.md)。
