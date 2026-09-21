---
name: sandrone
description: 使用 Sandrone 转换代理数据，查看或管理 Subscription 和 FileSpec，编写 processor 脚本并渲染客户端配置。
---

# Sandrone

优先使用用户指定的入口。未指定时，有 shell、`curl` 和 `SANDRONE_URL` 就使用
随附的 `scripts/sandrone-api.sh`；否则使用已连接的 Sandrone MCP。两者都不可用时，
说明如何设置 `SANDRONE_URL`、可选的 `SANDRONE_TOKEN`，或连接 MCP。

## 资料来源

- 构造请求前，从当前服务的 `/v1/schemas` 或 `sandrone://schemas` 读取所需
  schema；不要凭 Skill 中的静态描述猜字段。
- 仓库存在时优先读取同版本的本地文档。否则按需查看 GitHub 上的
  [文档索引](https://github.com/kuuvahki-labs/sandrone/blob/main/docs/README.md)、
  [HTTP API](https://github.com/kuuvahki-labs/sandrone/tree/main/docs/reference/http-api)、
  [MCP 参考](https://github.com/kuuvahki-labs/sandrone/blob/main/docs/reference/mcp.md)和
  [示例](https://github.com/kuuvahki-labs/sandrone/tree/main/examples)。部署版本不是
  `main` 时，尽量查看对应 tag 或 commit 的内容。
- 需要端点与资源操作映射时读取 [操作参考](references/workflows.md)。只获取当前任务
  所需的文档、schema 和资源定义。

## 写入与验证

只有明确的创建、更新或删除请求才授权持久化；查看、起草、preview 和 render 不授权
写入。覆盖或删除前读取现有定义，目标仍不明确时再询问。涉及写入或敏感数据时读取
[安全边界](references/safety.md)。

完成已授权写入后验证受影响行为：输入或 processor 变更通常执行 preview/render，
仅元数据变更可回读定义。按结构化 `code`、warning 和实际响应判断结果；写入成功但
验证失败时分别说明，不要为重试验证而重复写入。`body_omitted` 只表示正文因大小限制
被省略，不表示服务创建了文件或分享链接。
