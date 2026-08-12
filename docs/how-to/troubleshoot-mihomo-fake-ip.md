# 排查 Mihomo fake-IP 问题

适用症状：某个域名在启用 Mihomo fake-IP 后无法解析或连接，而停用 fake-IP 后恢复。本页只处理 DNS 映射相关问题，不把路由、代理策略和 TUN 问题统称为 fake-IP 故障。

当前默认值和匹配语义见 [Mihomo fake-IP 参考](../reference/mihomo-fake-ip.md)。

## 1. 确认症状

分别记录：

- 出错的精确域名；
- DNS 返回值是否落在 Mihomo fake-IP 地址池；
- 查询由哪个 resolver 处理；
- 后续连接是否经过同一个 Mihomo 实例；
- 失败发生在解析、建连还是路由阶段。

如果 DNS 查询经过 Mihomo、后续连接却绕过 Mihomo，应用可能拿到无法还原的 fake IP。反之，真实 IP 解析成功但路由仍失败，继续修改 `fake-ip-filter` 通常无效。

## 2. 检查已有规则

先确认候选域名是否已被现有条目覆盖：

- `"*"` 只匹配不含点的单标签名称；
- `+.example.com` 匹配根域名及各级子域名；
- `geosite:*` 的实际集合取决于运行时 geodata；
- `fake-ip-filter` 只控制 DNS 返回真实 IP，不等于 `DIRECT`。

查看 Sandrone 保存的基础 source：

```sh
curl -sS \
  -H "Authorization: Bearer change-me" \
  "http://127.0.0.1:1137/v1/files/config.yaml?mode=source&response=json"
```

如果文件来自 Web，编辑已有文件会保留其 source；新默认值不会自动回填旧文件。

## 3. 添加最窄修复

优先精确主机名，其次才是必要的域名后缀。以下使用保留域名演示写法：

```yaml
dns:
  fake-ip-filter:
    - "service.example.com"
```

确认同一后缀下多个主机都需要真实解析后，才扩大为：

```yaml
dns:
  fake-ip-filter:
    - "+.example.com"
```

不要为一个失败域名直接复制整份社区清单。规则越宽，越可能绕开本来能够正常工作的 fake-IP 映射。

在 Sandrone 中，保留基础配置的同时追加条目，可使用 file-stage `merge` 的 `yaml_override`，或在 Web 中添加对应的结构化处理器。处理器按 `processors` 声明顺序执行。

## 4. 重新加载并验证

渲染配置，确认条目实际出现在最终产物而非只存在于保存的 spec：

```sh
curl -fsS \
  -H "Authorization: Bearer change-me" \
  "http://127.0.0.1:1137/v1/files/config.yaml" \
  -o rendered.yaml
```

重新加载 Mihomo，清理系统和应用 DNS 缓存，然后重复原始查询。成功标准至少包括：

- 目标域名不再返回 fake-IP 地址；
- 应用能够完成原先失败的连接；
- 未被规则覆盖的普通域名仍按预期使用 fake IP。

如果第一项成功、第二项仍失败，转查路由、代理策略、防火墙和 TUN。

## Tailscale / MagicDNS

若故障涉及 Tailscale，先按
[社区配置预设](../reference/community-config-presets.md#tailscale-三态与安全边界)
核对模式、精确 DNS、TUN、规则与入站边界。本页只补充现场验证：渲染最终 YAML，
确认系统实际采用预期 resolver，并分别检查名称解析、目标路由和入站 ACL。

Windows 上若控制面域名曾返回 fake IP，可在重载配置后刷新 DNS 并重启 Tailscale：

```powershell
ipconfig /flushdns
Restart-Service Tailscale
Resolve-DnsName service.example.com
& 'C:\Program Files\Tailscale\tailscale.exe' status
```

不要把某次观察到的控制面 IP 固化为长期规则；按域名配置并以实际解析、连接结果验证。

## 仍未解决

收集以下最小诊断再继续定位：

- 脱敏后的最终 `dns`、`tun` 和相关 route 配置；
- 出错域名及是否返回 fake IP；
- DNS 查询与连接各自经过的接口；
- Mihomo、系统 resolver 和应用日志中的对应时间点；
- 加入规则前后的可重复结果。

诊断内容可能包含订阅 URL、节点地址或原始输入。公开前务必脱敏。
