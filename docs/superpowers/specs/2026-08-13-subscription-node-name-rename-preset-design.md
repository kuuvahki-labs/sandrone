# 订阅节点名称重命名预设设计

## 背景

当前本地数据中的 `rename.js` 以 `kind=static` 文件资源保存，订阅通过
nodes-stage `script` processor 的 `source.file` 引用它。Web 订阅处理器编辑器已经
提供创建期预设，但用户要先创建文件资源，才能使用这份节点名称规范化脚本。

本改动只增加一个 Web 创建期快捷入口。它不新增 processor type，也不迁移、修改
或删除任何已保存的文件资源和订阅。

## 用户行为

订阅处理器类型列表新增“节点名称重命名”。选择并添加后，Web 在当前处理链末尾
追加一个普通的 nodes-stage `script` processor：

- 名称为“节点名称重命名”；
- `params.source.type` 为 `inline`；
- `params.source.content` 为现有 `rename.js` 的脚本正文；
- 不生成 `params.args`，所有脚本参数继续使用脚本自身默认行为。

添加后的 processor 不再具有特殊身份。用户可以通过现有脚本编辑器修改内联脚本、
增加参数、调整顺序或删除它，保存时仍序列化为普通 `script` processor。

## 实现边界

脚本正文复制到订阅 feature 内受版本控制的独立 JavaScript 资源，通过 Vite `?raw`
导入。React 组件只负责把预设选项映射成 processor draft，不内嵌大段脚本文本。

预设沿用现有“过滤信息节点”的创建期合成选项模式：预设 ID 只存在于新增处理器的
类型选择器中，不会写入持久化 ProcessorSpec。后端 registry、脚本引擎、HTTP/MCP
契约和订阅存储格式均不改变。

## 兼容性

`data/files/rename.js.json` 及任何 `source.file = rename.js` 引用保持原样；本改动没有
运行时数据迁移。旧文件引用继续由现有受控文件脚本解析路径执行，新建预设则自包含
脚本正文，两条路径可以同时存在。

因为预设复制的是可编辑内联内容，后续 Web 版本更新脚本资源不会静默改写用户已经
保存的 processor。

## 验证

前端测试覆盖：

- 类型列表显示“节点名称重命名”；
- 添加预设会在原有处理器之后追加 nodes-stage `script`；
- 序列化结果使用内联 source，正文与受版本控制的脚本资源一致；
- 序列化结果不包含 `args`，也不依赖可选的脚本文件资源列表；
- 现有普通脚本和“过滤信息节点”预设行为保持不变。

交付前运行相关 Vitest、Web typecheck、Web lint 和 `git diff --check`。若范围内门禁发现
脚本本身与 lint 或打包规则冲突，再在不改变运行语义的前提下处理资源文件约束。
