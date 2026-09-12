# 节点探测

## 运行时观测边界

节点探测回答“这个节点在本次检查中是否可达、耗时多少”。它是运行时观测，不是 parser、renderer 或节点协议模型的一部分。

探测不会隐式修改输入节点；显式 probe processor 或脚本可使用观测过滤、排序或
写入 `NodeIR.Meta`。结果是一次运行时状态，不成为稳定协议字段。

领域关系见[领域模型](domain-model.md)，请求字段与入口用法见 [CLI 参考](../reference/cli.md)和 [Processor 参考](../reference/processors.md)。

## 包与编排职责

探测跨两个核心边界：

- `internal/service` 解析 `NodeInput`，应用项目设置中的运行默认值，校验节点，准备核心 payload，协调缓存，并汇总 report。
- `internal/probe` 注册和选择 backend，执行并发探测，返回节点结果和 backend report。

service 使用已注册 renderer 准备核心 payload 后交给 backend，避免另一套节点映射。
processor 和 script 使用注入的 probe runner；包依赖见[架构总览](overview.md#依赖边界)。

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

service 先应用项目设置的 probe defaults，再规范化 method 和 core。归一化后的请求同时用于 backend 选择、payload 准备和缓存匹配，避免同一调用在不同阶段采用不同默认值。

method 选择以下探测行为：

- `tcp_connect` 直接连接节点 endpoint，不使用 core。
- `udp_ntp` 通过节点出站发送 NTP 请求，目前使用 sing-box。
- `url_test` 通过节点出站访问 HTTP URL，支持 sing-box 和 Mihomo。

省略 method 时使用 `url_test`；省略需要的 core 时使用 `sing-box`。

`udp_ntp` 通过节点出站发送 NTP 请求，用来观察 UDP 链路，不等价于 HTTP 代理语义。`url_test` 通过选定核心访问测试 URL，比 TCP connect 覆盖更多协议握手与转发路径。

请求指定 core 时，只能选择该 core 注册的 backend。所需 backend 未编译、core 不存在或多个候选无法唯一选择时返回结构化错误，不静默降级为 TCP；否则“探测成功”会表达另一种语义。

核心启动或 payload 准备失败同样是整次调用错误。节点级连接失败则保留为
`alive=false` 结果，使调用方能够区分“backend 无法运行”和“backend 成功观察到节点
失败”。选定核心的 renderer 无法表达某个节点时返回 `probe_node_unsupported`；它同样
保持 `alive=false`，但明确表示 backend 没有执行该节点，不能当作不可达结论。

## 执行与并发

probe engine 按规范化 method 执行整个节点批次。backend 负责单节点 timeout、attempts 和 concurrency 限制；context 取消会停止等待中的工作。

结果保持输入节点顺序。backend 返回致命错误时整次调用失败，不发布不完整结果。

`tcp_connect`、`udp_ntp` 和不同核心的 `url_test` 测量口径不同。调用方比较 duration 时必须同时检查 method、core 和 backend，不能把它们视为同一基准。

## Store-backed cache

probe cache 是 service 管理的逐节点内部 TTL cache。它只在已保存资源的执行作用域
内持久化；临时 diagnose、inline convert 和未保存草稿会正常探测，但不读写持久
cache。它不是写入 `NodeIR` 的状态，也不是长期历史数据库。

缓存按“资源 key → 探测参数 selector → `ConnectionKey` → 观测”组织。
资源 owner 可以是已保存 subscription 或 file；[key 布局与 TTL](storage.md#key-布局)
由存储层统一管理。selector 包含 method、core、backend 名称与版本、有效目标参数、
timeout、attempts 和结果语义版本；连接字段决定 `ConnectionKey`。

名称、标签、metadata、来源格式、数组顺序和 concurrency 不改变观测身份。
连接或探测参数变化会使相应观测 miss，不会为同一资源另建持久 key。

命中语义按连接逐项组合：

- 同一批次可以同时包含命中与未命中节点，只对未命中的连接调用 backend。
- 同批重复 `ConnectionKey` 只探测一次，再把观测绑定回各自的当前 `RuntimeID` 和名称。
- 返回结果仍按当前输入顺序排列；每项独立标记 `CacheHit`，report 记录命中数。
- 缓存观测保留原检查时间，避免把读取时间误当成探测时间。
- 当前请求的 dependencies、source refs 和节点上下文仍按本次物化结果重建。

过期、损坏或读取失败按 miss 处理。cache 写入失败不会把已经完成的探测改成失败，但会产生 `probe_cache_write_failed` warning。

同一 selector 更新时以当前连接集合替换观测，其他 selector 随资源 key 的 TTL、
`refresh` 或显式清理失效。不同资源不复用相同连接的 probe 观测；组合订阅的子调用
仍使用各自 owner。缓存不保存长期历史，持久化与刷新边界见[存储架构](storage.md#cache)。

## Report 与失败语义

每个 `NodeProbeResult` 记录节点身份、method、core、backend、存活状态、duration、
检查时间和可选错误。`ProbeReport` 分别汇总成功、不支持、失败和 cache hit；
`error_counts` 只统计真正的失败，并按 method、core 提供 dimensions。

service 汇总输入、validation、renderer 和 backend 的诊断，以及 dependency/source
trace。单节点失败和不支持保留在 results；backend 未注册、启动失败、payload 整体
构造失败或引擎结果无效属于调用级错误。report 随调用返回，不写回资源定义。

## 安全与日志

probe backend 不需要任意宿主文件访问，也不会从 processor 接收通用网络或进程能力。真实代理测试只使用 service 准备的节点 payload 和请求目标。

正常完成日志只记录 method、core、计数、缓存状态和耗时，不记录节点 payload。
report 可能携带上游原始输入，不能假定已脱敏；处理要求见[错误与诊断参考](../reference/errors.md)。

错误码与 warning wire 语义见[错误与诊断参考](../reference/errors.md)，探测结果字段以 [`internal/domain/probe.go`](../../internal/domain/probe.go) 为准。
