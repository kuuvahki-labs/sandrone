# 节点预览测活摘要设计

## 目标

当节点预览结果包含测活元数据时，在列表卡片未展开状态直接显示当前节点的
存活状态和测活耗时，让用户可以快速横向比较节点，同时保留现有节点差异与
元数据详情。

## 范围

- 只消费 preview 响应中节点已有的 `probe.*` meta。
- 不新增后端接口，不额外发起测活，也不改变 preview、probe 或缓存语义。
- 不增加按测活状态筛选、按耗时排序或快慢阈值。
- 不把 `duration_ms` 定义为 RTT；界面和可访问文案统一称为“测活耗时”。

## 数据模型与解码

在 subscription preview 前端模型中增加可选的类型化测活摘要：

```ts
interface SubscriptionPreviewProbe {
  alive: boolean;
  durationMs?: number;
  method?: string;
  checkedAt?: string;
  errorCode?: string;
}
```

`previewNodeFromAPI` 从节点的 `meta` 中读取 `probe.alive`、
`probe.duration_ms`、`probe.method`、`probe.checked_at` 和
`probe.error_code`，并生成 `SubscriptionPreviewNode.probe`。原始节点仍完整保留在
`raw` 中，继续供现有“元数据”详情使用。

解码遵循以下边界：

- 只有 `probe.alive` 严格等于字符串 `"true"` 或 `"false"` 时才认为存在测活摘要；
- `probe.duration_ms` 只有在可解析为正整数时才写入 `durationMs`；缺失或非法值不
  映射成 `0`；
- 其他字段只在非空时保留；非法或不完整元数据不得导致预览失败。

列表只展示 `diff.after` 的测活摘要。`removed` 节点没有当前处理后状态，不回退到
`diff.before`，以免把旧测活结果表现为当前状态。

## 列表呈现

现有卡片标题区域由“节点摘要 / 展开箭头”两列调整为“节点摘要 / 测活摘要 /
展开箭头”三列。测活摘要靠右、禁止换行，长节点名继续在左侧可用空间内换行：

```text
香港 01                     ● 42 ms   ▾
vmess · hk.example.com:443
```

显示规则：

| 数据状态 | 列表显示 |
| --- | --- |
| `alive=true` 且有合法 `durationMs` | 成功状态点 + `42 ms` |
| `alive=true` 且无合法耗时 | 成功状态点 + `可用` |
| `alive=false` | 错误状态点 + `失败` |
| 没有有效测活摘要 | 不渲染测活区域，也不保留占位 |

数字使用 tabular numerals，便于纵向比较。颜色只表达可用或失败，不根据毫秒值
划分绿、黄、红等级，避免把不同 method 的耗时套入同一性能阈值。

测活摘要的悬停提示补充 method、测活时间和失败 error code 中已有的字段；
这些信息仍可在展开后的“元数据”详情中完整查看。卡片的可访问描述同步包含
“测活耗时”“可用”或“失败”，不能只依赖颜色或悬停提示表达状态。

## 组件边界

- codec 负责把开放的 `meta` 字符串映射为受控的前端测活摘要；React 组件不直接
  解释 `raw.meta`。
- 一个局部的测活摘要组件只负责状态到文案、颜色和提示内容的映射。
- `PreviewNodeCard` 只负责布局和把当前 `after` 节点的摘要传给展示组件。
- 现有 diff、metadata、warning 和 preview 状态筛选行为保持不变。

## 异常与边界行为

- `duration_ms` 缺失、为 `0`、负数或非数字时，不显示 `0 ms`。
- 失败结果即使带有耗时，也以“失败”为主要列表状态；耗时可留在详情中。
- method、测活时间或 error code 缺失时，提示只省略对应片段。
- 超长节点名、窄屏和多位耗时不得让测活摘要与展开箭头重叠。

## 验证

- codec 纯逻辑测试覆盖成功、失败、无耗时、非法耗时、缺少 alive 和原始 meta
  保留。
- 页面组件测试覆盖 `42 ms`、`可用`、`失败`、无摘要以及只读取 `after` 的行为。
- 可访问性断言验证测活状态不只通过颜色表达，并能随节点详情操作被读出。
- 保留现有节点差异、元数据切换、warning 和状态筛选测试。
- 实施后先运行相关前端窄测，再运行与 Web 改动匹配的 typecheck、lint、测试和
  build 门禁。
