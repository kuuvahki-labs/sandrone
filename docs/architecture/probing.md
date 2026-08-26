# 节点探测

## 运行时观测边界

节点探测回答“这个节点在本次检查中是否可达、耗时多少”。它是运行时观测，不是 parser、renderer 或节点协议模型的一部分。

三个模型的生命周期不同：

- `NodeIR` 表达可转换的节点配置。
- `NodeProbeResult` 表达单个节点的一次观测。
- `ProbeReport` 汇总一次调用的后端、成功/失败、缓存命中和错误维度。

探测不会隐式修改输入节点。只有调用方显式使用 nodes-stage `probe` processor 或脚本时，结果摘要才可以用于过滤、排序或写入 `NodeIR.Meta`；这些 metadata 仍不是稳定协议字段。

领域关系见[领域模型](domain-model.md)，请求字段与入口用法见 [CLI 参考](../reference/cli.md)和 [Processor 参考](../reference/processors.md)。

## 包与编排职责

探测跨两个核心边界：

- `internal/service` 解析 `NodeInput`，应用项目设置中的运行默认值，校验节点，准备核心 payload，协调缓存，并汇总 report。
- `internal/probe` 注册和选择 backend，执行并发探测，返回节点结果和 backend report。

其它层只承担窄职责：

- `internal/domain` 定义请求、结果、错误码和 report。
- node processor 和 script 使用 service 注入的 probe runner。
- entrypoint 只做参数、鉴权和响应呈现。

`internal/probe` 不 import adapter、processor 或 entrypoint。需要 Mihomo 或 sing-box 配置片段时，service 使用已注册 renderer 生成 payload，再交给 backend；backend 不自行建立第二套节点映射。

## Backend 与 build tag

所有构建都注册轻量 `tcp_connect` backend。它直接连接节点的 server 与 port，可以发现基础 TCP 不可达，但不能证明认证、TLS、Reality、WebSocket、QUIC 或代理协议完整可用。

可选核心由 build tag 加入构建图：

- `probe_mihomo` 注册 Mihomo `url_test`。
- `probe_singbox` 注册 sing-box `url_test` 和 `udp_ntp`。

这些核心作为 Go 依赖编译进当前进程，不调用宿主机上的 mihomo 或 sing-box
可执行文件。Mihomo 和 sing-box 的 URL backend 都直接调用内嵌核心的测速
API，不启动本地控制接口。

裸 `go build` 或 `go test` 不带 tag 时只有 TCP backend。仓库的 Makefile 默认使用 `probe_singbox`，发布 Docker 构建也包含 sing-box backend；因此“模块在 `go.mod` 中”与“能力进入这次二进制”是两个不同事实。

当前构建实际注册的 backend 由 runtime capability summary 给出，架构文档不维护运行实例清单。

## Backend 选择

service 先应用项目设置的 probe defaults，再规范化 method 和 core。归一化后的请求同时用于 backend 选择、payload 准备和 cache key，避免同一调用在不同阶段采用不同默认值。

method 是唯一的探测行为选择器，必须显式属于以下三种：

- `tcp_connect` 直接连接节点 endpoint，不使用 core。
- `udp_ntp` 通过节点出站发送 NTP 请求，目前使用 sing-box。
- `url_test` 通过节点出站访问 HTTP URL，支持 sing-box 和 Mihomo。

省略 method 时使用 `url_test`；省略需要的 core 时使用 `sing-box`。

`udp_ntp` 通过节点出站发送 NTP 请求，用来观察 UDP 链路，不等价于 HTTP 代理语义。`url_test` 通过选定核心访问测试 URL，比 TCP connect 覆盖更多协议握手与转发路径。

请求指定 core 时，只能选择该 core 注册的 backend。所需 backend 未编译、core 不存在或多个候选无法唯一选择时返回结构化错误，不静默降级为 TCP；否则“探测成功”会表达另一种语义。

核心启动或 payload 准备失败同样是整次调用错误。节点级连接失败则保留为 `alive=false` 结果，使调用方能够区分“backend 无法运行”和“backend 成功观察到节点失败”。

## 执行与并发

probe engine 按规范化 method 执行整个节点批次。backend 负责单节点 timeout、attempts 和 concurrency 限制；context 取消会停止等待中的工作。

结果保持输入节点顺序。backend 返回致命错误时整次调用失败，不发布不完整结果。

`tcp_connect`、`udp_ntp` 和不同核心的 `url_test` 测量口径不同。调用方比较 duration 时必须同时检查 method、core 和 backend，不能把它们视为同一基准。

## Store-backed cache

probe cache 是 service 管理的逐节点内部 TTL cache。它只在已保存资源的执行作用域
内持久化；临时 diagnose、inline convert 和未保存草稿会正常探测，但不读写持久
cache。它不是写入 `NodeIR` 的状态，也不是长期历史数据库。

每个 cache key 基于：

- 规范化节点按需计算的 `ConnectionKey`。
- 规范化的 method、core，以及实际选择的 backend 名称和版本。
- 应用默认值后的 URL、NTP server、预期状态等目标参数。
- 影响单次探测语义的 timeout 与 attempts。

节点名称、标签、metadata、来源格式、数组顺序和 concurrency 不改变连接或探测目标，
因此不进入 key。连接字段、有效探测参数或核心版本变化会形成新 key。

命中语义按连接逐项组合：

- 同一批次可以同时包含命中与未命中节点，只对未命中的连接调用 backend。
- 同批重复 `ConnectionKey` 只探测一次，再把观测绑定回各自的当前 `RuntimeID` 和名称。
- 返回结果仍按当前输入顺序排列；每项独立标记 `CacheHit`，report 记录命中数。
- 缓存观测保留原检查时间，避免把读取时间误当成探测时间。
- 当前请求的 dependencies、source refs 和节点上下文仍按本次物化结果重建。

过期、损坏或读取失败按 miss 处理。cache 写入失败不会把已经完成的探测改成失败，但会产生 `probe_cache_write_failed` warning。

缓存只保存单节点观测、backend 信息和可重新绑定的 renderer warning 模板；它是内部加速数据，不会成为可列举、可分享的 report 资源。Store 备份也排除 cache 前缀，见[存储架构](storage.md)。

缓存 key 按保存资源隔离为 `probe/subscriptions/<name>`；Store-backed Cache 将其
映射为 `cache/probe/subscriptions/<name>.json`。probe 业务把完整 value 解码为
“探测参数 selector -> ConnectionKey -> 节点观测”的 typed 结构；selector 包含
method、core、URL、backend 等有效探测语义。同一 selector 更新时以当前连接集合
全量替换该组，因此已经移除或连接发生变化的节点不会继续留在当前组。其它 selector
不做猜测性清理，由 TTL、`refresh` 或显式缓存清理结束生命周期。内部记录没有独立
TTL，整个 key 统一过期，普通全量保存不能延长已有绝对过期时间。组合订阅递归执行
时，B、C 自身的处理结果仍分别写入自己的 key；只有 A 自己发起的组合后探测才写入
A，不跨资源复用相同连接。

## Report 与失败语义

每个 `NodeProbeResult` 记录节点身份、method、core、backend、存活状态、duration、检查时间和可选错误。`ProbeReport` 汇总成功、失败、cache hit 和 error code，并按 method、core 提供 dimensions。

service 在 cache miss 时合并：

- 输入解析和节点校验 warnings。
- 为核心准备 payload 时 renderer 产生的兼容 warnings。
- backend 的节点失败和能力 warnings。
- dependency 与 source trace。

单节点失败保留在 results 中并对应结构化 warning。backend 未注册、核心启动失败、payload 无法安全渲染或引擎返回无效结果属于调用级错误，不伪装成成功的空结果集。

report 随本次结果返回，不写回 subscription 或 file。内部 cache 可以暂存单节点观测，但不提供长期审计、趋势或历史窗口；需要监控历史时应由外部可观测系统消费结果。

## 安全与日志

probe backend 不需要任意宿主文件访问，也不会从 processor 接收通用网络或进程能力。真实代理测试只使用 service 准备的节点 payload 和请求目标。

backend 自己构造的节点 warning 只需要最小身份上下文；但上游 parser warning 可能附带原始 URI 行或结构化值，其中可能有 password、UUID、token 或 private key。因此 probe report 不能被假定为已统一脱敏。

正常完成日志只记录 method、core、计数、缓存状态和耗时，不记录节点 payload。调用方在持久化、公开返回或集中记录 report 前仍须执行部署自己的敏感信息策略。

错误码与 warning wire 语义见[错误与诊断参考](../reference/errors.md)，探测结果字段以 [`internal/domain/probe.go`](../../internal/domain/probe.go) 为准。
