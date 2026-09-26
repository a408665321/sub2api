---
name: maintain-photonthinx-sub2api
description: Maintain the Photonthinx internal Sub2API fork, including upstream upgrades, monthly usage reports, regression checks, internal release tags, Nexus publishing and deployment rollback. Use for this internal fork, not unrelated Sub2API installations.
---

# Photonthinx Sub2API 内部版维护

先定位用户指定的 Sub2API checkout，读取该仓库 `docs/photonthinx-maintenance.md`，其中维护社区基线、内部功能、两个报表接入点和发布流程。本 Skill 源码位于仓库 `.agents/skills/maintain-photonthinx-sub2api`；安装目录可能是符号链接，不能把它当作仓库根。

## 基线与边界

- 内部主干是 `internal/main`，当前升级起点为 `v0.2.8-photonthinx.1`，社区基线 `v0.2.8`。以后以仓库维护清单和实际 tag 为准。
- 不把混有用户改动的 `main` 或旧 `custom/main` 当作发布起点。保留用户未提交文件，使用独立功能分支/worktree。
- 保留 OIDC 独占登录和客户端取消后的 response 归属绑定修复；不恢复旧内部 forward-audit 定制。
- 接到实现/诊断任务不等于获得镜像发布、公开仓库推送或生产升级授权；按当次用户范围执行。Skill 不额外要求已获授权的操作再次确认。

## 升级与新增定制

社区升级先在临时分支验证，默认合并正式社区 tag 保留历史；rebase 仅在临时分支演练，不默认强推共享主干。对照维护清单确认内部功能仍生效，更新社区基线和接入点清单。

新增内部代码优先使用独立 `photonthinx` 文件/目录。月报仅在 `backend/internal/server/routes/admin.go` 和 `frontend/src/views/admin/UsageView.vue` 接入；保持现有 DI、数据库、网关记录、计费及原统计 API 兼容。不要因避免冲突而绕过管理员鉴权、限流或合规中间件。

## 报表不变量

- 固定 `Asia/Shanghai` 自然月/日，左闭右开；默认实际费用降序，同值总 Token 降序、用户 ID 升序。
- 全站汇总不受列表搜索和分页影响；份额以全站实际费用为分母；同一月报使用单次 SQL 快照。
- 计入零费用和已停用/删除用户的存量记录。请求数是用量行数，月活去重用户，Token 包含输入/输出/缓存创建/缓存读取。
- 账号成本沿用已有快照公式，金额 SQL 聚合/排序前不舍入。不要用原 Top 200 API 替代完整分页。
- 曲线按需加载，忽略过期请求；失败显示重试，不伪装为零用量。未知历史留空，当前周期标明未完整，不画未来日期。
- 历史基于现存记录；规划时最早为 2026-09-03，不能假设之前是零，也不能恢复已删除数据。
- 部署保留期至少 400 天，保留已有更长值，billing dedup 不短于 usage logs。用仓库 retention 工具从核实的有效值生成 override，不能盲目用 400 覆盖 730。

## 回归、发布、部署

按维护文档执行报表统计/权限/竞态测试、原 UsageView/排行测试、OIDC 和 response 绑定回归、前端类型检查及生产构建。确认集成测试实际启动临时数据库，避免把 Docker 缺失的跳过当作通过。无数据库迁移；既有源码差异仅两个接入点。

发布使用 `v<社区版本>-photonthinx.<递增号>` Git tag，镜像不带前导 `v`。核验版本基线、提交、测试、目标 tag 和现存镜像。Nexus 发布复用已安装 `photonthinx-docker-publish` Skill，不自行绕过其目标校验；构建、发布、部署分清各自请求范围。凭据由现有凭据管理提供，不写入源码或 Skill。

生产升级前核实主机、Compose 目录、当前镜像和保留配置，保存可回滚的旧引用与 override。只重建应用，保留 PostgreSQL/Redis；验证版本、健康端点、OIDC、月报与绑定修复。回滚只恢复应用版本和适用配置，不缩短保留期；社区迁移另做兼容性评估。HTTP registry 问题不自动触发 Docker daemon 重启。
