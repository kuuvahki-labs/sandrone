# 安全策略

## 支持版本

首次 tagged release 发布前，最新的 `main` commit 是接收安全修复的支持目标。开始发布 tagged release 后，除 release notes 另有说明，最新 tagged release 将成为支持目标。请在报告中提供受影响的 tag 或能够定位问题的 commit。

## 报告漏洞

请不要通过公开 issue 披露安全漏洞。请使用 GitHub private vulnerability reporting 向仓库维护者提交报告。

提交报告时，请尽量包含以下信息：

- 受影响版本或 commit。
- 可复现步骤，包括必要的命令、API route 或触发条件。
- 影响范围，例如可导致的数据泄露、权限绕过、拒绝服务或其他风险。
- 已脱敏的日志、配置和输入样例。

请不要提交真实订阅、token、cookie、私有节点 URI、生产环境密钥或未脱敏日志。维护者可能会在 GitHub private vulnerability reporting 线程中要求补充信息或验证修复。
