# 部署到 Vercel 与 Cloudflare R2

本页说明单租户 Sandrone 的 serverless 部署。Vercel Go Function 运行现有
HTTP、MCP 与 Web UI；Cloudflare R2 保存项目设置、资源、分享、备份数据和
Store-backed cache。

## 准备 R2

1. 在 Cloudflare 创建一个私有 R2 bucket。
2. 创建只允许该 bucket object read/write 的 R2 API token。Sandrone 不需要
   bucket administration、公开 bucket、CORS 或 lifecycle 权限。
3. 记录 account ID、access key ID 和 secret access key。

Preview 与 Production 必须使用不同 bucket，或至少使用不同 prefix。不要让
开发预览和正式服务同时写入同一个 namespace。

## 配置 Vercel 环境变量

在对应 Vercel Environment 中设置：

```text
SANDRONE_TOKEN=<随机的长 bearer token>
SANDRONE_STORAGE_BACKEND=s3
SANDRONE_S3_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
SANDRONE_S3_REGION=auto
SANDRONE_S3_BUCKET=<bucket>
SANDRONE_S3_PREFIX=sandrone/
SANDRONE_S3_FORCE_PATH_STYLE=false
SANDRONE_S3_ACCESS_KEY_ID=<R2 access key ID>
SANDRONE_S3_SECRET_ACCESS_KEY=<R2 secret access key>
```

使用临时凭据时再设置 `SANDRONE_S3_SESSION_TOKEN`。不要把上述值写进
`vercel.json`、仓库文件、部署日志或 shell history。

Vercel profile 强制要求非空 `SANDRONE_TOKEN` 和 S3 后端。配置缺失时 Function
返回脱敏的初始化错误，不会退化到临时文件系统。

## 部署与检查

连接仓库并创建 Preview Deployment。部署成功后依次检查：

1. `GET /version` 和 Web 首页可以加载。
2. 未携带 bearer token 的 `/v1/*` 与 MCP 请求被拒绝。
3. 新建并读取 subscription、file 和 settings。
4. 创建公开 share，并通过 `/s/<id>` 读取。
5. 执行一次 conversion 和 MCP discovery。
6. 创建新的 deployment 或触发 cold start 后，前述数据仍存在。
7. 导出并恢复一个小型 backup。

HTTP 与 MCP 的请求格式分别见
[HTTP API](../reference/http-api/README.md)和[MCP 参考](../reference/mcp.md)。

## Serverless 能力边界

Vercel 不运行进程内 scheduler，也不提供任何 probe backend：

- `scheduler.enabled=false`；
- `probe.enabled=false`；
- `core.mihomo=false`；
- `core.sing_box=false`。

Web UI 会隐藏定时更新、测活默认值和新增 probe processor 的入口。直接调用
`/v1/probe`、MCP probe tool、脚本 probe 或执行已有 probe processor 时返回
`probe_backend_unavailable`，不会静默跳过。

CLI、Docker 与其他长驻 `serve` 部署不受此 profile 影响。

## 限制与维护

Vercel Functions 的请求和响应 payload 上限为 4.5 MB。超过该限制的 render、
MCP inline output 或 backup 即使未达到 Sandrone 自身上限，也会被平台拒绝。
当前版本不提供绕过该限制的旁路协议。

Coordinator 只协调单个 warm instance。多个 Function instance 共享 R2 时，
backup restore 仍是 best-effort、非事务操作。执行 restore 前应进入维护窗口，
停止其他 writer。

从 filesystem 迁移时：

1. 停止源服务的写入；
2. 导出小于平台限制的 backup；
3. 部署使用全新 R2 namespace 的 Preview；
4. 恢复 backup 并验证资源；
5. 再把正式流量切到新 deployment。

## 回滚

应用代码异常时，将 Vercel 流量回滚到上一 deployment。只要旧版本理解当前
Store wire format，就继续使用同一 Production R2 namespace；不要同时让新旧
deployment 写入。

若存储数据本身需要回退，应在维护窗口中恢复事先验证的 backup。不要通过删除
bucket 或批量删除 prefix 代替应用级恢复。
