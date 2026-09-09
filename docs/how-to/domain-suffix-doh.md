# 按域名后缀使用自定义直连 DoH

使用文件处理器脚本
[`domain-suffix-doh.js`](../../examples/scripts/domain-suffix-doh.js)，
让指定域名及其子域名通过自定义 DoH 解析。脚本支持 Mihomo、使用新版 typed DNS
配置的 sing-box 1.12+ 和 Shadowrocket；其他文件类型会明确报错。
Shadowrocket 分支添加 Host DoH 映射和直连规则，目标网站也会直连。

## 在 Web UI 使用

1. 打开需要修改的 Mihomo、sing-box 或 Shadowrocket 文件。
2. 在**文件阶段**添加「脚本」处理器，放在会修改 DNS 或规则的其他处理器之后。
3. 选择内联来源，粘贴示例脚本全文。
4. 在脚本参数中每行填写一个 `key=value`：

```text
suffixes=example.com,example.net
doh=https://dns.example.org/dns-query
```

把示例域名和 DoH 地址换成自己的值。地址只填写 HTTPS URL，不附加 `#DIRECT`。
脚本的完整参数与输入限制由[源码头部说明](../../examples/scripts/domain-suffix-doh.js)
维护。

sing-box 使用域名形式的 DoH 地址时，需要已有 DNS server 解析 DoH 服务器本身。
脚本默认使用当前模板的 `dns-cn`。如该 tag 已更名，补充：

```text
bootstrap=your-existing-dns-tag
```

应选择能够独立解析 DoH 主机名的直连 DNS server，避免循环依赖。
DoH 地址直接使用 IP 时不需要 bootstrap；证书仍需对该 IP 有效。

## 多个文件复用

把脚本正文保存为名为 `domain-suffix-doh.js`、类型为 `static` 的 Sandrone 文件资源。
然后在不同客户端文件的处理器中选择文件来源，引用同一个资源名：

```yaml
type: script
stage: file
params:
  source:
    type: file
    name: domain-suffix-doh.js
  args:
    suffixes: [example.com, example.net]
    doh: https://dns.example.org/dns-query
```

这里的 `name` 是已登记资源名，不能直接填写本机脚本路径。
登记方法见[编写 processor 脚本](write-processor-script.md#nodes-stage给节点名加前缀)。

## 预览与边界

保存前预览最终正文，检查：

- Mihomo 的 `dns.nameserver-policy` 中出现 `+.example.com` 和 `+.example.net`，
  值为自定义 DoH 加 `#DIRECT`；`direct-nameserver-follow-policy` 为 `true`。
  该开关会让现有的其他 nameserver-policy 也参与直连出口解析；更具体的域名策略
  仍按 Mihomo 优先级生效。
- sing-box 的 `dns.servers` 中出现对应 HTTPS server，`dns.rules` 第一条是后缀
  规则。脚本生成的新版 HTTPS server 使用默认直连 dialer，不继承 `route.final`。
  以域名为地址时还会写入 `domain_resolver`。
- Shadowrocket 的 `[Host]` 出现域名本身和 `*.域名` 两条 `server:https://...`
  映射，`[Rule]` 最前面出现 `DOMAIN-SUFFIX,域名,DIRECT`。原有相同后缀规则会被
  替换成这条优先的直连规则，其他 Host、规则与全局 DNS 配置保留。

Mihomo / sing-box 分支不修改网站流量的路由，也不强制代理端远程解析改成本地解析；
Shadowrocket 分支会让目标域名的网站流量直连。浏览器自带的 DoH 需要另外处理。
Mihomo 的节点域名专用 DNS 配置保持原样。重复执行不会追加相同的 sing-box server
或 Shadowrocket Host/规则；在同一个 sing-box 文件内配置多组后缀/DoH 时，每个实例应指定不同
`tag`，并把更具体的后缀实例放在后面，使其规则优先。

Shadowrocket 完整文件中生成的结果如下（域名与地址为占位示例）：

```ini
[Host]
example.com = server:https://dns.example.org/dns-query
*.example.com = server:https://dns.example.org/dns-query

[Rule]
DOMAIN-SUFFIX,example.com,DIRECT
```

在 Sandrone 的 `ini_override` 补丁中，`[Host+]` 表示追加到已有 `[Host]`。
本脚本直接修改完整文件，生成合并后的 `[Host]`，不用额外添加合并处理器。
追加语法见 [merge](../reference/processors.md#merge)。

配置生成不代表 DoH 网络可达；应在实际使用客户端的网络中验证查询成功。

语义依据：[Mihomo DNS](https://wiki.metacubex.one/config/dns/)、
[sing-box HTTPS DNS](https://sing-box.sagernet.org/configuration/dns/server/https/)
和 [DNS rule](https://sing-box.sagernet.org/configuration/dns/rule/)。
