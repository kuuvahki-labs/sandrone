# 第一次使用 Web UI

本教程带你在 Web UI 中保存一份本地订阅、把它加入组合订阅，并生成一份可复制的 Mihomo 配置。整个过程不需要手写 HTTP 请求或 JSON。

示例只使用保留域名和占位凭据，不会连接真实节点。Docker Compose 会把数据保存在 `sandrone-data` volume 中。

## 准备

需要：

- Docker；
- 一个 Sandrone 仓库 checkout；
- 可访问 `http://127.0.0.1:1137` 的浏览器。

在仓库根目录启动服务：

```sh
docker compose up --build
```

构建完成后，打开 <http://127.0.0.1:1137>。登录页出现时，在“管理员 token”中输入：

```text
sandrone
```

然后选择“进入”。这个 token 只用于本地开发；对外提供服务前必须通过 `SANDRONE_TOKEN` 更换。

## 1. 新建本地订阅

从左侧导航进入“订阅”，选择右下角“新建订阅”，再选择“本地”。

填写：

- 名称：`demo-local`
- 内容：

```text
ss://aes-128-gcm:demo-password@node-a.example.com:8388#Node-A
ss://aes-128-gcm:demo-password@node-b.example.com:8389#Node-B
```

格式保持“自动”，然后选择“保存”。保存成功后，`demo-local` 会出现在“我的订阅”中。

## 2. 创建组合订阅

再次选择“新建订阅”，这次选择“组合”。

填写：

- 名称：`demo-all`
- 包含订阅：选择 `demo-local`

组合订阅可以把多个本地或远程订阅汇总成一个节点集合。本例只包含一个来源，先保持流程可重复、结果容易检查。

## 3. 添加处理链

在同一页面的“处理链”中选择“添加处理器”：

1. 类型选择“名称处理”；
2. “重命名方式”选择“添加前缀”；
3. “内容”填写 `demo-`。

再添加一个处理器：

1. 类型选择“排序”；
2. “排序字段”填写 `+name`。

处理器按页面中的声明顺序执行。保存组合订阅后，`demo-all` 会出现在订阅列表中。

## 4. 预览节点

打开 `demo-all` 的编辑页面，选择“预览”。在“节点预览”中检查：

- “处理前”和“处理后”都为 `2`；
- 节点名称为 `demo-Node-A` 和 `demo-Node-B`；
- 本例通常没有预览警告。

预览会读取来源并运行处理链，但不会修改已经保存的订阅定义。

## 5. 新建 Mihomo 配置

从左侧导航进入“文件”，选择右下角“新建文件”，再选择“mihomo 配置”。

填写文件名称，例如 `demo.yaml`。在“配置生成”中：

1. “节点来源”选择 `demo-all`；
2. 等待页面显示“已加载 2 个节点”；
3. “配置模板”保持默认的“标准”。

模板会生成代理组、规则集和规则的基础结构；仍可在页面中继续调整。选择“保存”完成创建。

## 6. 查看生成结果

在文件列表中打开 `demo.yaml`，选择“预览”。“最终文件内容”中应能看到：

- `proxies` 中的两个示例节点；
- 模板生成的 `proxy-groups`；
- 完整的 `rules`。

使用内容区域右上角的复制按钮即可复制生成结果。如果页面显示警告，先阅读警告详情，再决定是否把配置交给客户端使用。

## 停止服务

回到运行 Docker Compose 的终端，按 `Ctrl+C` 停止服务。需要同时删除本教程产生的数据时，再运行：

```sh
docker compose down --volumes
```

该命令会删除 Compose 管理的 `sandrone-data` volume，其中可能包含你此前保存的其它本地数据；执行前先确认不再需要。

## 下一步

- 要生成 sing-box 或 Shadowrocket 配置，见[渲染客户端配置](../how-to/render-client-config.md)。
- 要理解订阅、处理链和文件的底层字段，见 [FileSpec](../reference/file-spec.md) 与 [Processors](../reference/processors.md)。
- 要通过程序管理 Sandrone，见 [HTTP API](../reference/http-api/README.md) 或 [MCP](../reference/mcp.md)。
