# 生成内置规则集目录

运行 `make ruleset-catalog`，根据
`internal/tools/ruleset-catalog-gen/sources.lock` 记录的提交生成被 Git 忽略的
`catalog.json.gz`。正式构建会嵌入该文件；缺少它时普通 `go build` 仍能成功，但规则集
目录接口会返回不可用。

手动发布工作流会在创建 Git tag 前更新并提交 lock 文件。tag 触发的 CI 在构建发布产物
和容器镜像时生成目录。生成器从 `MetaCubeX/meta-rules-dat` 锁定的 `meta`、`sing` 提交，以及
`blackmatrix7/ios_rule_script` 锁定的 `master` 提交中读取 URL 元数据；后者只处理
`rule/Shadowrocket` 子目录，并选择各分类 README `使用说明` 中列出的 `.list` 文件，
检查其声明的 `RULE-SET` 或 `DOMAIN-SET` 形态，以及内容是否仅包含 IP 规则。

快照保存经过排序、去重的元数据和实时分支 URL，不保存规则正文。因此，目录成员仅在
来源 lock 文件变化时更新，而通过这些 URL 获取的内容仍会跟随上游分支变化。

将 `RULESET_CATALOG_GITHUB_MIRROR` 设置为 GitHub 镜像的基础 URL，可让构建阶段通过
镜像克隆；未设置时使用 `https://github.com`。无论是否使用镜像，生成的原始内容
URL 都保持为官方地址。

本地 CLI 构建会把两个上游裸仓库存放在
`${XDG_CACHE_HOME:-$HOME/.cache}/sandrone/ruleset-catalog`，再次构建相同锁定提交时
无需重新获取。可用 `RULESET_CATALOG_CACHE_DIR` 修改缓存位置。Docker 不使用该 CLI
缓存，而是独立使用目录构建阶段的 BuildKit 层缓存。

上游项目及贡献者保留署名。Blackmatrix 声明采用 GPL-2.0，并聚合了各自带有署名的
规则来源；使用或再分发相关规则文件时，应查阅对应许可证与署名要求。
