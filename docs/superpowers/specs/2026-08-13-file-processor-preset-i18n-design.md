# 文件处理器预设参数化与国际化设计

## 目标

统一 Web 内置文件处理器预设的名称生成方式，并将 GitHub 规则源地址替换改成可编辑的通用字符串替换脚本：

- 新建文件和新添加预设时，processor `name` 使用当前 Web 界面语言；
- 已保存文件中的 processor 名称保持原样，不做隐式迁移；
- 产品名、协议名和技术缩写保留原文，只翻译动作与说明；
- GitHub/jsDelivr 映射从脚本源码移入 processor 参数；
- 不新增后端 processor type，不改变 file-stage `script` 的执行契约。

## 名称契约

`FileProcessorPreset.labelKey` 同时作为下拉标签和新生成 processor 的默认名称来源。预设构建函数接收当前 `Translator`，由调用方在创建默认 processor、添加直接预设和补齐依赖预设时统一传入。

中文名称保留 `GitHub`、`TUN`、`DNS`、`QUIC`、`UDP/P2P`、`MPTCP`、`Fake-IP`、`IPv4`、`Tailscale`、`Tailnet` 等专业词，翻译其余动作。例如：

- `Traditional NTP Direct` → `传统 NTP 直连`；
- `QUIC Fallback` → `QUIC 强制回退`；
- `Windows Relaxed Route` → `Windows 宽松路由`；
- `Sniff & DNS Hijack` → `流量嗅探与 DNS 劫持`；
- `GitHub Rule Source Rewrite` → `GitHub 规则源镜像替换`。

英文翻译保持自然英文。`TUN`、`Sniffer` 等无需中文化的现有标签保持不变。

已有文件进入编辑器时直接使用已保存的 `name`。预设识别继续只比较 `type` 与 `params`，不因语言或用户改名而失效，也不会在保存时自动重写历史名称。

## 通用字符串替换脚本

现有 `github-rule-source-rewrite.js` 改为用途无关的字符串替换脚本。脚本读取：

```yaml
args:
  preset_id: github-rule-source-mirror
  replacements:
    - [source, destination]
```

`replacements` 是有序的二元组数组。脚本按数组顺序对 `input.file.content` 做全量字面量替换；不使用正则表达式。参数缺失、不是数组或元素不是两个字符串时显式失败，避免静默生成错误配置。

所有读取 `input.args` 的 Web 内置预设脚本都必须在文件头用简短注释列出参数名、格式和用途。无参数脚本不添加空的参数说明。字符串替换脚本的头部至少说明：

```javascript
// Parameters:
// - preset_id: stable preset identifier; request args must not override it.
// - replacements: ordered array of [source, destination] string pairs.
```

参数注释只描述用户编辑脚本时必须知道的契约，不复制完整 processor 或 scripting API 文档。

GitHub 快捷项只负责提供默认参数，包含当前三组 GitHub Raw → jsDelivr 映射。用户可在现有脚本参数编辑器中修改映射，脚本源码不再承载目标地址。

新快捷项的中英文名称分别为：

- `GitHub 规则源镜像替换`；
- `GitHub rule source mirror replacement`。

## 预设目录集成与兼容

GitHub 快捷项改为普通 `FileProcessorPreset`，供 Mihomo、sing-box 和 Shadowrocket 的 driver catalog 复用。`FileProcessorBuilder` 不再维护专属 kind 集合、选项值和添加分支；依赖排序、重复抑制及通知都走现有 preset planner。

新预设通过通用脚本源码和稳定的 `args.preset_id` 识别，允许用户修改 `replacements` 后仍保持预设身份。识别逻辑同时接受旧版带 `sandrone:file-preset=github-rule-source-rewrite` marker 的内联脚本，从而：

- 保留已有文件；
- 避免重复添加同一快捷项；
- 不主动改写旧脚本或旧 processor 名称。

这条旧格式识别是唯一兼容点，不保留旧构建路径或旧 UI 名称。

## 数据流

新建文件时，`FileFormFields` 将当前 translator 传给 driver 的默认 processor 工厂。用户从下拉菜单添加预设时，builder 将同一 translator 传给 preset planner；planner 再用它构建请求项及自动补齐的依赖项。因此所有创建路径使用同一套名称来源。

编辑已有文件时，解码、展示和保存继续保留资源中的名称。用户主动添加的新预设使用当前语言，但同一文件中允许保留历史语言名称。

## 验证

测试覆盖：

- 通用脚本按顺序完成多组字面量替换，并拒绝非法参数；
- GitHub 预设把三组映射写入 `replacements`，源码不含这些地址；
- 所有读取 `input.args` 的内置预设脚本都有与实际参数一致的文件头说明；
- 新格式和旧 marker 格式都能识别，修改替换表后仍能抑制重复；
- 中文和英文界面新增的所有文件预设使用对应本地化名称；
- 新建三种 typed file 的默认 processor 名称按界面语言生成；
- 编辑已有文件时不改写其 processor 名称；
- GitHub 快捷项不出现在 static file 中。

文档同步更新 `docs/reference/processors.md` 与 `docs/reference/community-config-presets.md`，只在 processor 参考中完整说明通用参数和兼容边界。

实现后先运行相关 Vitest 窄测，再运行 Web `lint`、`typecheck` 和全仓 `make check`。
