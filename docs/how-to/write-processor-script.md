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

引用文件脚本时，Sandrone 会先渲染目标文件资源，再把最终正文作为脚本执行。
若脚本文件本身需要渲染参数，把字符串键值放在 `params.source.args`；当前脚本的
业务参数仍放在 `params.args`。两组参数彼此不继承：

```json
{
  "source": {
    "type": "file",
    "name": "generated-script.js",
    "args": {"variant": "production"}
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
