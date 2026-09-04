# 节点信息 HTTP API

本页定义从一个临时完整 `NodeIR` 按需派生 URI 或 IP 归属信息的接口。它不保存节点，
也不创建 Share。

## POST `/v1/nodes/inspect`

该管理接口受 bearer token 保护。请求体包含一个完整 `NodeIR`，以及非空、无重复的
`include` 封闭集合；当前值为 `uri` 和 `ip`：

```json
{
  "node": {
    "name": "fixture-node",
    "type": "trojan",
    "server": "proxy.example.com",
    "port": 443,
    "password": "fixture-password"
  },
  "include": ["uri"]
}
```

只请求 `uri` 时，服务通过现有 `uri-list` renderer 处理该 `NodeIR`，返回 URI 和
renderer warning，不经过 JSON Nodes 解析或前端协议拼接：

```json
{
  "uri": {
    "value": "trojan://fixture-password@proxy.example.com:443#fixture-node",
    "warnings": []
  }
}
```

只有 `include` 包含 `ip` 时才解析和查询 IP。`node.server` 必须是字面 IPv4、IPv6
或 DNS 名称，不能包含 scheme、端口、路径、userinfo 或空白。字面 IP 直接使用；
域名通过运行时 resolver 解析，按 resolver 顺序规范化和去重，只使用第一个地址。
IPv4-mapped IPv6 规范化为 IPv4。

公开地址会返回国家、洲与 ASN 信息：

```json
{
  "ip": {
    "server": "proxy.example.com",
    "ip": "8.8.8.8",
    "ip_version": 4,
    "public": true,
    "country_code": "US",
    "country": "United States",
    "continent_code": "NA",
    "continent": "North America",
    "asn": "AS15169",
    "as_name": "Google LLC",
    "as_domain": "google.com",
    "source": {
      "name": "ipwho.is",
      "url": "https://ipwho.is"
    }
  }
}
```

DNS 与归属结果均不缓存。私网、保留地址、文档地址和 Mihomo
`198.18.0.0/15` fake-IP 只返回本地分类结果：

```json
{
  "ip": {
    "server": "198.18.0.1",
    "ip": "198.18.0.1",
    "ip_version": 4,
    "public": false
  }
}
```

错误契约：

| HTTP status | code | 条件 |
| --- | --- | --- |
| `400` | `invalid_argument` | `include` 为空、重复或包含未知值，或 IP 查询的 `server` 非法 |
| `500` | `render_failed` 等 | 请求的节点不能渲染为单个 URI |
| `502` | `ip_lookup_failed` | DNS 无结果、解析失败、超时，或 `ipwho.is` 返回失败/畸形响应 |

最小请求：

```sh
curl -sS -X POST "$SANDRONE_URL/v1/nodes/inspect" \
  -H "Authorization: Bearer $SANDRONE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"node":{"name":"fixture-node","type":"trojan","server":"proxy.example.com","port":443,"password":"fixture-password"},"include":["ip"]}'
```
