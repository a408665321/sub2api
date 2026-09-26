# Photonthinx 内部版维护

## 分支和基线

- 社区基线：正式 tag `v0.2.8`（`fd80b08c9`）。
- 当前已发布内部基线：`v0.2.5-photonthinx.3`。
- 当前开发版本：`v0.2.8-photonthinx.1`；功能分支：`codex/photonthinx-v0.2.8`。
- 内部主干：`internal/main`。
- `main`、`origin/main` 和旧 `custom/main` 不作为内部版本的隐式发布起点。保留其原状。
- `origin` 是社区仓库；`mine` 是个人 GitHub 仓库。内部主干的远端目标应由用户确认，不能直接假设可推送到公开仓库。

## 内部功能清单

| 功能 | 来源/文件 | 升级回归要求 |
| --- | --- | --- |
| 客户端取消后保存 response 归属绑定 | `d1cf23e33`；`backend/internal/service/openai_ws_state_store.go` | `TestOpenAIWSStateStore_*`，特别是两个 `PersistsAfterParentCancellation` 测试 |
| OIDC 独占登录 | `1b3d82cd6`；该提交中的 auth/config/frontend 文件 | OIDC 独占/运行时关闭/注册及登录页面回归；不得意外恢复密码入口 |
| 管理员月报 | `backend/internal/photonthinx/usagereport`；各层 `photonthinx_usage_report*.go`；前端 `photonthinx` 模块 | 月界、汇总、分页、权限、竞态、失败重试 |
| OpenAI 重置卡窗口策略与观察模式 | `backend/internal/service/photonthinx_openai_auto_reset_policy.go`；`frontend/src/components/account/PhotonthinxOpenAIAutoResetPolicy.vue` | Pro 无 5h、5h 禁用、7d 保护期、观察去重、执行前复核、模式切换 |
| 用量保留至少 400 天 | `deploy/photonthinx` | 不缩短更长配置；billing dedup >= usage logs；检查磁盘增长 |
| 内部维护 Skill | `.agents/skills/maintain-photonthinx-sub2api` | 格式校验；更新基线、清单和命令 |

不恢复旧的内部 forward-audit 定制。社区版本自带的其它能力按基线保留，不在月报任务中顺带删除。

月报相对内部起点，仅修改两个既有源码：

1. `backend/internal/server/routes/admin.go` 调用 `registerPhotonthinxUsageReportRoutes`。必须位于现有管理员鉴权、面板限流、审计及合规中间件保护下。
2. `frontend/src/views/admin/UsageView.vue` 增加标签、懒加载和显示切换；原筛选组件使用 `v-show` 保留内部状态。

其余均为新增文件；无迁移、原 Repository 接口/构造器扩展或网关、计费链路改动。独立报表接口通过实际 repository 的附加方法连接已有 DI。升级时若上游改变连接或 Handler 结构，优先修新模块的适配。

## OpenAI 重置卡策略

`.3` 不新增表或 migration，不扩展 Repository、DI 或 API 路由。账号配置和运行态复用 `accounts.extra` JSONB，完整决策历史复用管理员审计日志，跨实例去重复用现有幂等表。

管理员配置键：

```json
{
  "photonthinx_auto_reset_credit_policy": {
    "mode": "observe",
    "reset_5h_enabled": false,
    "reset_7d_enabled": true,
    "seven_day_guard_days": 2
  }
}
```

- 未包含策略对象的旧账号保持 `.2` 行为：`enforce`、5h/7d 均启用、保护期为 0。
- 新启用自动用卡默认 `observe`；切换 `enforce` 必须在管理端确认。
- `seven_day_guard_days` 范围 0～7，管理端步进 0.5；距 7d 自然重置时间小于或等于保护期时不使用卡。
- 只有新鲜上游快照明确识别出约 5h（4～6 小时）或约 7d（6～8 天）窗口才参与判断。套餐名称不参与判断；缺少时长、无效重置时间和过期快照均 fail closed。
- 观察模式模拟真实调度：有完整可用卡时可越过普通暂停阈值，到用卡阈值记录 `would_reset` 后暂停，不调用兑换接口。
- 5h 用卡关闭或 7d 进入保护期时，即使普通自动暂停被禁用，窗口耗尽也强制暂停。

服务管理、账号编辑请求不得回写的运行态键：

- `photonthinx_codex_window_presence`：每个新鲜快照同时写入 5h/7d 的存在与不存在状态，覆盖旧残留值。
- `photonthinx_auto_reset_observation_state`：每个窗口仅保留最近一次判定，不保存数组。

审计动作 `system.openai.reset_credit.policy` 不含卡 ID、兑换 ID、Token 或凭据。去重键包含账号、窗口、上游周期、判定、原因和策略摘要；幂等设施不可用时不降级为重复普通日志。主要接入点限定为：

1. `openai_quota_auto_reset_config.go`：配置校验和运行态剥离。
2. `openai_gateway_usage.go`：新鲜窗口存在性落盘。
3. `openai_gateway_scheduling.go`：暂停/放行策略。
4. `openai_quota_auto_reset.go`：观察、兑换前二次读取与 fail-closed。
5. `admin_account.go`：全量编辑时保留服务管理运行态。
6. `EditAccountModal.vue`：挂载独立策略组件。

## 报表 API 与口径

`GET /api/v1/admin/internal/reports/usage/monthly`

- `month=YYYY-MM`，省略时取北京时间当前月；拒绝未来月份。
- `page` 默认 1；`page_size` 默认 20、最大 100；`search` 搜索名称/邮箱文字或精确用户 ID。
- `sort_by=actual_cost|requests|total_tokens`；`sort_order=asc|desc`；默认实际费用降序，同值总 Token 降序、用户 ID 升序。
- 返回 `summary`（全站）、`items`（本页）、`total`（搜索匹配用户数）和保留历史元数据。汇总、分页总数和列表在同一 SQL 快照完成，即使空页也返回全站汇总。

`GET /api/v1/admin/internal/reports/usage/trend`

- `month` 为终止月份；`granularity=day` 限单月，`granularity=month&months=12` 最多 12 个月。
- 可选正整数 `user_id`；省略为全站。用户曲线按需加载，一次只展开一个用户。
- 返回补齐周期的 `points`；`available=false` 的历史区间在图上留空，不能画作零；`partial=true` 表示首个不完整周期或尚未结束周期。
- 所有范围采用 `Asia/Shanghai`，左闭右开，当前月截至查询时刻，未来日期不绘制。

实际费用汇总 `actual_cost`；账号成本使用 `COALESCE(account_stats_cost,total_cost) * COALESCE(account_rate_multiplier,1)`。SQL 使用原 NUMERIC 精度进行加总、排序和份额计算，API 沿用项目浮点数约定，显示最多 6 位小数，不在聚合前舍入。

请求数为已保存的 API 用量行数，不承诺全部成功，也不是网页访问量。总 Token = 输入 + 输出 + 缓存创建 + 缓存读取。月活为当月 distinct user ID。保留零费用和停用/软删除用户，关联用户信息缺失时仍按 ID 归属。搜索不改变顶部汇总，费用占比始终以全站实际费用为分母。

每次查询有 30 秒上限，SQL 使用时间谓词和可选用户谓词；现有 `created_at` 与 `(user_id,created_at)` 索引可用。无 Top 200 限制、不下载原始明细、不新增永久聚合表。列表和曲线是独立请求，实时新记录到达时允许出现时间点差异。单个月报内部汇总/列表保证同快照；一致性测试用固定测试事务。

`available_from` 是全站最早现存记录，不是完整性承诺。不能推断或恢复被手工清理的中间历史。页面始终显示“基于现存用量记录”；早于现存起点的月份标记数据不足。2026-09-18 规划时线上起点为 `2026-09-03 13:15:45.813197+08`，不硬编码到程序，查询动态读取。

## 保留配置

社区默认原始记录 90 天、计费去重 365 天。报表读取原始记录；旧每日全站聚合不能重建已清理的个人历史。保留期调整只保护现存和新增记录，不能补回以前的数据。

部署前检查当前生效的环境变量和挂载的 `config.yaml`，环境变量优先。只查看两个相关键，避免把完整 Compose 环境/凭据贴到日志中：

- `dashboard_aggregation.retention.usage_logs_days`
- `dashboard_aggregation.retention.usage_billing_dedup_days`

两项现值都不大于 400 时可使用 `deploy/photonthinx/compose.retention.yaml` 作为最后一个 Compose override。已有更长设置时，根据核实的生效值生成独立 override，例如：

```sh
python3 deploy/photonthinx/retention.py --usage-logs-days 730 --usage-billing-dedup-days 800
```

输出为 730/800，保留更长配置；例如 730/400 会得到 730/730。工具要求显式输入生效值，零值/禁用清理语义需先人工核实，不能直接转换为开启清理。生成结果只包含两个环境变量，不含凭据，也不自动部署。不要同时加载默认 400 override 覆盖生成结果。

将选定 override 加入当前完整 Compose 文件列表，先运行 `docker compose ... config --quiet`，核对最终两个环境变量再升级应用。保持其他清理开关和保留期不变。关注 usage_logs 和索引的磁盘增长；本地配置的交付不代表线上已经应用。

## 社区升级

1. 确认当前生产 tag、内部主干、工作区状态，创建临时 worktree/升级分支。
2. 获取并核验目标正式社区 tag，比较 release diff、配置和迁移。
3. 默认在临时分支将正式 tag 合并到内部主干后验证。需要 rebase 时仅在临时分支演练；不默认强推重写共享主干。
4. 对照上方功能及两个接入点清单，检查上游是否已有等效修复，避免丢失或重复修补。
5. 通过验证后合入 `internal/main`；更新本文件的社区基线、内部版本、保留配置说明。

## 验证

在 backend 执行（Go 版本按 go.mod）：

```sh
go test -tags unit ./internal/photonthinx/... ./internal/handler/admin ./internal/server/routes ./internal/service ./internal/handler ./internal/server/middleware -run 'Photonthinx|ParseShanghai|FillTrend|OIDC|Usage|OpenAIWSStateStore' -count=1
go test -tags integration ./internal/repository -run 'TestPhotonthinx|TestUsageLog_' -count=1
go test -tags unit ./internal/service -run 'OpenAIAutoReset|Photonthinx' -count=1
```

集成测试使用 testcontainers 的临时 PostgreSQL/Redis，不连接生产库。Docker 不可用时上游 harness 会跳过，本地需要确认测试实际执行。可用 `SUB2API_TEST_POSTGRES_IMAGE=postgres:18-alpine` 指定测试镜像。

在 frontend 执行：

```sh
pnpm test:run src/components/admin/photonthinx src/views/admin/__tests__/photonthinxUsageView.spec.ts src/views/admin/__tests__/UsageView.spec.ts src/components/admin/usage/__tests__/UserTokenRanking.spec.ts
pnpm test:run src/components/account/__tests__/PhotonthinxOpenAIAutoResetPolicy.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts
pnpm typecheck
pnpm build
```

还需执行 `python3 -m unittest discover -s deploy/photonthinx -p 'test_*.py'` 和 skill-creator 的 `quick_validate.py`。核对 `git diff --name-status v0.2.8`，并确认没有意外迁移或发布文件残留旧版本号。

## 版本发布与回滚

本功能实现阶段只交付可审查代码、主干、配置和 Skill。发布/生产升级必须属于当次用户授权范围。

- 版本模式：Git `v<社区版本>-photonthinx.<递增号>`，镜像去掉前导 `v`。不覆盖既有 tag，不自动用 `latest`。
- 从核验的 `internal/main` 提交构建，记录提交、基线、构建参数、镜像标签/digest及测试。构建参考已有 `Dockerfile`、`deploy/Dockerfile.photonthinx-runtime`，实际构建方式须与发布版本内容一致。
- Nexus 推送/核验复用 `photonthinx-docker-publish` Skill。Hosted: `repo.photonthinx.careray.com:8082/docker-hosted/photonthinx/sub2api:<version>`；Group 拉取路径为 `docker-group/photonthinx/sub2api:<version>`。
- 不在 Git、Skill 或输出中写入凭据；使用现有登录或凭据管理 Skill。
- 最近部署参考：主机 `photonthinx-1`（对话曾写 phtonthinx-1），Compose 目录 `/srv/sub2api`；发布前重新核实。升级前必须现场读取当前镜像，不依赖文档中的历史版本。
- 升级前保存旧镜像引用、完整有效 Compose 文件列表和 override 备份，校验 400 天配置。只对应用执行 `docker compose ... up -d --no-deps sub2api`，保持 PostgreSQL/Redis。
- 核验容器健康、程序版本、直连及 Nginx `/health`、OIDC 独占登录、月报权限/汇总、response 绑定取消回归。
- 该主机历史上直接拉取 HTTP registry 有 HTTPS 匹配问题。若仍存在，使用已授权的 `docker save`/`docker load` 分发流程；不为此自动重启 Docker。
- 回滚到记录的旧镜像及原部署配置，只重建应用；不要把已提高的保留期降回更短值。月报无 schema 迁移，无需回滚数据库。社区升级如含迁移，需另行评估数据库回滚兼容性。

## Skill 源码与安装

源码：`.agents/skills/maintain-photonthinx-sub2api/SKILL.md`。本机安装目录：`~/.codex/skills/maintain-photonthinx-sub2api`。可安装为指向保留的内部 worktree 的符号链接；移除 worktree 前先迁移链接到稳定内部 checkout。Skill 变更随内部主干审查。
