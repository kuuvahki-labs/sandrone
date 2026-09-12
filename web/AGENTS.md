# Web UI 模块约定

开发与构建入口见 [Web README](README.md)，验证范围见
[贡献指南](../CONTRIBUTING.md#选择验证范围)。

## 所有权与依赖

- `app/root.tsx`、`app/routes.ts`、`app/routes/**` 装配路由，处理 route/search
  params 和 navigation；feature 页面通过 props/functions 接收所需数据与操作。
- `app/core/**` 拥有 providers、shell 与跨 feature 的应用装配。
- `app/features/<feature>/**` 拥有该功能的 model、data、components 与 pages。
- `app/shared/**` 提供不依赖具体 feature 的 API、storage、UI 与通用模型。

依赖沿 `root/routes/core -> features -> shared` 向内：feature 不依赖 sibling
feature 或入口，shared 不依赖 feature，core 不依赖 route，route 不引用另一 route。
这样内层逻辑可独立使用，跨功能协作由装配层完成。导入路径和转导出形式可按模块需要
选择，仍需避免循环依赖和反向依赖。

## 页面数据复用

列表缓存按服务与会话隔离，写入后失效；恢复或会话变化后，旧请求不能覆盖新状态。
过期列表可保留显示并后台刷新，失败不能使旧结果重新变成新鲜结果。
编辑读取权威详情，服务端负责最终校验；列表摘要不能代替完整定义。
缓存时长与实现见 [resource-list-cache.ts](app/shared/api/resource-list-cache.ts)。

## FileDriver 边界

- driver core 提供纯类型、registry、codec 与策略工具，不依赖具体 driver 或 UI。
- 具体 driver 负责目标格式、默认值与编译策略；registry 组合 driver，不依赖 UI。
- editor 组合 domain adapter 与 presentation slots；UI registry 只挂接 UI bundle，
  raw-only driver 无需 UI bundle。
- 通用配置模型不依赖具体 fields 或 UI registry，客户端 wire 格式由 driver codec
  处理，避免界面保存时绕过转换。
- 未知或缺失 kind 保留原始定义并只读，避免编辑器丢失无法理解的数据；服务端最终校验。

## 测试

按可观察行为选择测试层，不用文件数量、行数、import/export 清单或写法快照限制扩展。

- `*.test.ts` 在 Vitest node 环境运行纯逻辑；`*.test.tsx` 和 `*.dom.test.ts`
  在 jsdom 中验证依赖 DOM、browser API 或 React 生命周期的行为。
- `app/test/integration/routing` 使用真实路由覆盖导航、鉴权、请求与保存流程。
  公开 URL 的预期独立于生产配置维护，防止配置和测试一起漂移。
- `app/test/architecture` 检查依赖方向、循环和纯模型边界，生产代码不得引用测试模块。
- `e2e` 使用 Playwright 验证 built SPA 的用户流程与响应式行为。

长期保留能防止数据丢失、竞态、契约或用户行为回归的测试；删除只复述旧实现的断言。
文档与生成产物约定见[贡献指南](../CONTRIBUTING.md#文档政策)和 [Web README](README.md)。
