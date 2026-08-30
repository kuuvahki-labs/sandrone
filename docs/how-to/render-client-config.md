# 渲染客户端配置

本页说明如何把一个已保存的订阅渲染进 Mihomo、sing-box 完整配置，以及如何
生成与订阅节点分离的 Shadowrocket `.conf`。假设服务已经启动，且订阅
`default` 可以成功 preview。

完整字段定义见 [FileSpec 参考](../reference/file-spec.md)；这里仅保留可直接改造的最小范式。

## 选择 canonical kind

目标与 `kind` 必须一一对应：

| 客户端 | `kind` | 常用文件名 |
| --- | --- | --- |
| Mihomo | `mihomo` | `config.yaml` |
| sing-box | `sing-box` | `config.json` |
| Shadowrocket | `shadowrocket` | `config.conf` |

`kind` 区分大小写且不能省略。不要把 renderer 名称（例如 `mihomo-proxies`）写成 file kind。

Web 模板的跨客户端规则顺序、DNS 隐私边界和中国 IP 兜底见
[客户端路由、DNS 与 IP 兜底](../reference/client-routing-dns.md)。

以下命令假设：

```sh
export SANDRONE_API="http://127.0.0.1:1137"
export SANDRONE_TOKEN="change-me"
```

## Mihomo

保存：

```sh
curl -sS -X POST "$SANDRONE_API/v1/files" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "config.yaml",
    "kind": "mihomo",
    "source": {},
    "config": {
      "subscriptions": ["default"],
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
    "processors": []
  }'
```

渲染：

```sh
curl -fsS \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  "$SANDRONE_API/v1/files/config.yaml" \
  -o config.yaml
```

## sing-box

sing-box 的 group、rule set 和 rule 使用对象结构：

```sh
curl -sS -X POST "$SANDRONE_API/v1/files" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "config.json",
    "kind": "sing-box",
    "source": {},
    "config": {
      "subscriptions": ["default"],
      "settings": {
        "groups": [{
          "type": "selector",
          "tag": "Proxy",
          "outbounds": ["$nodes", "direct"]
        }],
        "rule_sets": [],
        "rules": [{"outbound": "Proxy"}]
      }
    },
    "processors": []
  }'
```

渲染并检查 JSON：

```sh
curl -fsS \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  "$SANDRONE_API/v1/files/config.json" \
  -o config.json
sed -n '1,40p' config.json
```

## Shadowrocket

Shadowrocket 以 INI source 为基础，driver 会清空 `[Proxy]` 并生成受管的分组与
规则 section。节点不写入 `.conf`，而是在 Shadowrocket 中通过
`shadowrocket-proxies` 分享链接单独添加：

```sh
curl -sS -X POST "$SANDRONE_API/v1/files" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "name": "config.conf",
    "kind": "shadowrocket",
    "source": {
      "type": "inline",
      "content": "[General]\nipv6 = false\n"
    },
    "config": {
      "settings": {
        "groups": [{
          "name": "Proxy",
          "type": "select",
          "proxies": ["PROXY", "DIRECT"]
        }],
        "rule_sets": [],
        "rules": ["FINAL,Proxy"]
      }
    },
    "processors": []
  }'
```

对应节点订阅可使用分享 URL 的 `format=shadowrocket-proxies`；它返回与
`mihomo-proxies` 同形的 Clash YAML。配置文件和节点订阅是两个独立入口。

渲染：

```sh
curl -fsS \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  "$SANDRONE_API/v1/files/config.conf" \
  -o config.conf
```

## 验证与定位失败

保存后先读取 spec，确认服务保存的是预期定义：

```sh
curl -fsS \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  "$SANDRONE_API/v1/files/config.yaml?mode=spec"
```

再请求带诊断的渲染结果：

```sh
curl -fsS \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  "$SANDRONE_API/v1/files/config.yaml?response=json"
```

常见问题：

- `kind` 缺失、大小写错误或使用了 renderer 名称；
- `config.settings` 不是 JSON object，或包含目标 driver 不认识的字段；
- Mihomo/sing-box 的 `config.subscriptions` 引用了不存在或无法生成节点的订阅，
  或 Shadowrocket FileSpec 携带了非空 `config.subscriptions`；
- 分组、规则或规则集互相引用的名称不一致；
- typed file 的 `config.settings.groups`、`rule_sets`、`rules` 不能省略；需要空输出时显式提交 `[]`。

需要结构化修改编译结果时使用 file-stage `merge`；只有开放式逻辑才使用 `script`。两者都写在 `processors` 中，并按声明顺序执行。详情见 [processor 参考](../reference/processors.md)。
