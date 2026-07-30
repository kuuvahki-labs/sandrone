# Web UI 模块约定

本文件只定义 `web/` 的长期模块与测试边界。安装、开发、构建和验证命令见
[Web UI 快速说明](README.md)；仓库级规则以根目录
[AGENTS.md](../AGENTS.md) 和 [CONTRIBUTING.md](../CONTRIBUTING.md) 为准。

## Runtime

Web UI 使用 React Router framework mode，并以 `ssr: false` 构建为静态
SPA。构建脚本会移除 server 产物，client-side 静态资源生成在
`web/build/client`；Sandrone HTTP server 从复制后或显式配置的静态目录提供
这些资源，不运行 Node application server。

`web/build/` 与复制到 `internal/entry/webui/static` 的资源都是生成物，不应提交。

## 所有权层次

- `app/root.tsx`、`app/routes.ts` 和 `app/routes/**` 是最外层 entry
  adapters，负责应用装配、route/search params、navigation 和窄接口转换。
  route 文件应提供真实 default component，不作为 forwarding re-export。
- `app/core/**` 拥有 providers、shell 与 application composition；跨 feature
  协作在这里或 route adapter 中通过窄 props/functions 完成。
- 每个 `app/features/<feature>/**` 纵向拥有自己的 model、data、components
  和 pages。feature 内不读取 route params，也不负责应用导航。
- `app/shared/**` 只容纳 feature-neutral 的 API、storage、UI、model 与通用算法。

## 依赖方向

依赖只能沿 `root/routes/core -> features -> shared` 向内：

- `shared/**` 只能依赖 `shared/**`。
- feature 只能依赖自身与 `shared/**`，不得依赖 sibling feature、`core` 或 route。
- `core/**` 可以依赖 core、features 与 shared，但不得依赖 route 或 root。
- route adapter 可以依赖 core、features 与 shared，但不得依赖另一 route。
- `root.tsx` 与 `routes.ts` 只向下装配，内层模块不得反向依赖入口。

跨目录 import 使用 `~/...` 直接指向具体模块；相对 import 只用于同一目录。
生产层不建立 barrel、不使用 `export *`，也不保留 compatibility forwarding
module。registry 组合值，不充当 re-export surface。

路由契约变化时，同时更新 `app/routes.ts` 与 integration harness 中独立维护的
期望；期望不得从生产 route config 自动生成。

## FileDriver 边界

- `features/files/drivers/core/**` 只持有纯类型、registry constructor、codec
  与 strategy helpers，不依赖 React、MUI、editor、config UI 或具体 driver。
- 各具体 driver 只组合自己的 definition、adapter、defaults 与 strategies，
  不导入 presentation fields。`drivers/registry.ts` 是 domain driver 与 create
  preset 的唯一 composition root，不依赖 UI。
- `editor/file-driver-ui.ts` 定义 presentation slots；
  `editor/file-driver-ui-registry.ts` 只挂接 structured driver 的 UI bundle。
  它是 editor-only registry，不是第二套 driver、profile 或 codec registry。
- domain adapter 与 UI bundle 只在表单组合层汇合。raw-only driver 不要求 UI
  bundle；config components 不查询 UI registry，也不导入具体 fields。
- 未知或缺失 kind 保留原始定义并只读；service typed-file driver 始终执行最终
  权威校验。

## 测试放置

- owner-specific focused tests 与源码 colocate；`*.test.ts` 使用 Vitest
  `node`，覆盖纯 model、config、graph 与静态契约。
- 前端测试优先覆盖纯逻辑，并尽量将业务规则、状态转换和序列化提取为可在
  `node` 环境运行的测试；只有契约确实依赖 DOM、browser API、React 生命周期
  或可访问交互时，才使用 jsdom/UI 测试。
- TDD 阶段可以用临时测试驱动实现；功能稳定后，应删除仅证明旧实现、样式细节
  或已被更低层契约完整覆盖的一次性测试，保留用于防止数据丢失、竞态、边界漂移
  和用户行为回归的长期测试。
- React component、page 和 focused route/module 测试使用 `*.test.tsx`
  与 `jsdom`；无 JSX 但依赖 browser API 的测试使用 `*.dom.test.ts`。
- `app/test/integration/routing` 使用真实 root 与 route modules，覆盖跨路由导航、
  request/auth/save 流程和 route-config drift。
- `app/test/architecture` 只验证全应用静态策略，例如 layer、cycle、direct
  import、route 与 FileDriver 契约，不承载用户行为测试。
- `e2e` 使用 Playwright 验证 built SPA 的 smoke 与响应式行为。

## 文档卫生

- runtime、所有权、路由、FileDriver 或用户可见行为变化时，同步检查本文件、
  [Web UI 快速说明](README.md) 以及 [公开文档索引](../docs/README.md) 中直接相关
  的页面。
- 长期文档只正向描述当前契约；命令清单留在 README，route/driver 等枚举留在
  代码与测试，不在多个页面复制。
- 实施中的 spec/plan 可以临时存在，但交付前删除；不保留完成记录、旧入口、
  compatibility 墓碑或 agent 执行清单。
- 示例必须脱敏；不提交真实订阅、节点 URI、token、cookie、私钥、私有 fixture、
  本机路径、agent/IDE 状态或生成产物。
