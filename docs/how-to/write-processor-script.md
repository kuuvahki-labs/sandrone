# 编写 processor 脚本

本页用两个可执行闭环演示 nodes-stage 与 file-stage 脚本。先完成[第一次使用 Web UI](../tutorials/first-web-ui.md)，保留其中的 `demo-all` 订阅和 Compose 服务，并设置：

```sh
export SANDRONE_API="http://127.0.0.1:1137"
export SANDRONE_TOKEN="sandrone"
```

完整 envelope、API 与 sandbox 契约见[脚本 API 参考](../reference/scripting-api.md)。

## Nodes-stage：给节点名加前缀

先把脚本保存为 `kind=static` 的受控文件资源：

```sh
curl -sS -X POST "$SANDRONE_API/v1/files" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "rename.js",
    "kind": "static",
    "source": {
      "type": "inline",
      "content": "function main(input, api) {\n  var prefix = input.args && input.args.prefix || \"\"\n  input.nodes = input.nodes.map(function (node) {\n    node.name = prefix + node.name\n    return node\n  })\n  return input\n}\n"
    }
  }'
```

预期返回 `{"ok":true}`。再创建一个 collection，在其处理链中引用脚本并传入 `prefix`：

```sh
curl -sS -X POST "$SANDRONE_API/v1/subscriptions" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "script-demo",
    "type": "collection",
    "inputs": [{
      "name": "base",
      "type": "subscription",
      "ref": {"kind": "subscription", "name": "demo-all"},
      "required": true
    }],
    "processors": [{
      "type": "script",
      "stage": "nodes",
      "params": {
        "source": {"type": "file", "name": "rename.js"},
        "args": {"prefix": "script-"}
      }
    }]
  }'
```

预览：

```sh
curl -sS -X POST \
  "$SANDRONE_API/v1/subscriptions/script-demo/preview" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{}'
```

响应中的 `after` 节点名应以 `script-` 开头。只需标准重命名、过滤或排序时，优先使用内建 processor。

## 完整示例：节点名称规范化

仓库提供了可直接运行的
[`examples/scripts/normalize-nodes.js`](../../examples/scripts/normalize-nodes.js)。它面向
Mihomo 与 sing-box 共用的 nodes-stage，在一个脚本中完成信息节点过滤、连接去重、
地区和线路识别、倍率提取、稳定排序、编号、协议标注及最终名称去重。脚本修改
保留节点的 `name`，并可选写入 `meta`；协议、安全层、传输层和 Flow 均读取
canonical `NodeIR`，不会从名称猜测连接配置，也不会访问网络。

在仓库根目录执行以下命令，将示例登记成 `kind=static` 的文件资源：

```sh
jq -Rs '{
  name: "normalize-nodes.js",
  kind: "static",
  source: {type: "inline", content: .}
}' examples/scripts/normalize-nodes.js |
curl -fsS -X POST "$SANDRONE_API/v1/files" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary @-
```

在 subscription 的 processors 中引用该文件：

```json
{
  "type": "script",
  "stage": "nodes",
  "params": {
    "source": {"type": "file", "name": "normalize-nodes.js"},
    "args": {
      "separator": " ",
      "protocol_mode": "main",
      "name_conflict": "drop"
    }
  }
}
```

默认名称类似 `🇭🇰 香港 01 IPLC 家宽 2× VLESS`。将 `separator` 设为
`" · "` 可改用圆点分隔；将 `protocol_mode` 设为 `detailed` 可输出
`VLESS Reality gRPC Vision` 一类详细协议组合。最终名称仍然重复时，默认保留
排序后的第一个节点并直接删除其余节点，从而保证 Mihomo proxy name 与 sing-box
outbound tag 唯一；设为 `name_conflict: "error"` 可改成显式失败。

`write_meta` 默认为 `false`，关闭时不会改写节点 `meta`。设为 `true` 后，最终保留
节点会写入 `normalize.*` 元数据，包括首次处理前的名称、地区、编号、线路、特征、
倍率、协议、安全层、传输层、Flow、IP 栈和来源标签。脚本保留其他已有 `meta`；
重复执行时不会覆盖最初记录的 `normalize.original_name`，其余派生字段按本次识别
结果刷新。

脚本支持的全部参数、模板变量和默认值写在文件头部。连接去重发生在命名之前，
因此两个原始名称相同但连接不同的节点仍会被编号成不同名称，不会被提前误删。

引用文件脚本时，Sandrone 会先渲染目标文件资源，再把最终正文作为脚本执行。
脚本文件资源自身的 processors 使用各自配置的参数，不继承当前 script processor
或请求的参数；当前脚本的业务参数放在 `params.args`：

```json
{
  "source": {
    "type": "file",
    "name": "normalize-nodes.js"
  },
  "args": {"prefix": "script-"}
}
```

## File-stage：补充 YAML 字段

保存一个读取当前文件、修改 `mixed-port` 再序列化的脚本：

```sh
curl -sS -X POST "$SANDRONE_API/v1/files" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "patch-yaml.js",
    "kind": "static",
    "source": {
      "type": "inline",
      "content": "function main(input, api) {\n  var document = api.yaml.parse(input.file.content)\n  document[\"mixed-port\"] = input.args && input.args.port || 7890\n  input.file.content = api.yaml.stringify(document)\n  return input\n}\n"
    }
  }'
```

创建 typed file，并在编译后的 file-stage 引用脚本：

```sh
curl -sS -X POST "$SANDRONE_API/v1/files" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "script-demo.yaml",
    "kind": "mihomo",
    "source": {},
    "config": {
      "subscriptions": ["script-demo"],
      "settings": {
        "groups": [{
          "name": "Proxy",
          "type": "select",
          "proxies": ["$nodes", "DIRECT"]
        }],
        "rule_sets": [],
        "rules": ["MATCH,Proxy"]
      }
    },
    "processors": [{
      "type": "script",
      "stage": "file",
      "params": {
        "source": {"type": "file", "name": "patch-yaml.js"},
        "args": {"port": 7891}
      }
    }]
  }'
```

渲染并检查参数已经生效：

```sh
curl -fsS \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  "$SANDRONE_API/v1/files/script-demo.yaml" |
  grep '^mixed-port: 7891$'
```

仅做 YAML、JSON 或 INI 的确定性结构合并时，优先使用 file-stage `merge`；脚本用于条件或计算逻辑。

## 查看 warning

脚本可通过 `api.warn({code, message, node, field})` 报告可恢复问题。例如：

```javascript
if (!input.nodes || input.nodes.length === 0) {
  api.warn({code: "empty_nodes", message: "no nodes available"})
}
```

warning 会出现在 subscription preview，或文件渲染的 JSON 包装响应中：

```sh
curl -sS \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  "$SANDRONE_API/v1/files/script-demo.yaml?response=json"
```

warning 不等于静默失败；processor 无法继续时，service 返回结构化错误。

## 失败排查

按以下顺序检查：

1. processor 的 `stage` 是否与脚本输入一致；
2. `params.source.type` 是否为 `file`，且脚本文件名存在；
3. `main(input, api)` 是否返回完整 envelope 或 `undefined`；
4. `args` 是否是 JSON object，字段类型是否符合脚本预期；
5. file-stage 脚本是否使用与目标内容一致的解析器；
6. preview、JSON 包装响应或 report 是否指出 processor 和错误码。

脚本运行在受控 sandbox 中。不要使用 `require`、Node.js 模块、任意文件系统、子进程、环境变量或通用网络访问；这些能力不会被注入。
