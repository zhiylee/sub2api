# 二开维护与社区同步

## 二开基本原则

以尽量小的侵入和后续同步成本实现需求。优先复用官方接口和执行流程，新增逻辑尽量放在
独立文件、组件中，在官方文件只保留必要接入点；避免复制整套实现、无关重构和大范围格式调整。
不能只按改动行数判断成本，还要检查行为、数据、缓存和发布渠道是否完整兼容。
每项功能独立记录来源、后续修复和退役条件；官方完整或部分覆盖后，及时删除或收窄补丁。

自有在线升级与官方发布信息的方案见
[升级渠道实现说明](backend/dev-docs/downstream-update-channels.md)；已接入当前分支，产物以正式 Release 为准。

## 当前基线

- 官方正式版为 `v0.2.25`，提交为 `348997267d756eab5729bb6b78a275a15d18796a`，包含 `v0.2.24`。 [官方 Release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.25)
- 集成分支为 `release/0.2.25`，当前二开版本为 `0.2.25-zy.2`，发布 tag 为 `v0.2.25-zy.2`。
- 二开来源为 `release/0.2.23` 的 `32339fac5`，包含 `v0.2.23-zy.2` 和之后的账号免费 Ultra Fast。

## 提交组织

按沿用顺序迁移出站档位计费、账号 Fast、Codex 上下文、hotfix 镜像、自有升级、逐模型倍率和账号免费 Ultra Fast。
自有升级提交包含运行逻辑、发布和安装适配。Go 1.27.4 由官方模块、Docker、CI 和发布流程提供。

每个功能提交使用稳定的 `Downstream-Patch` 标识。以下命令可按功能定位迁移后的提交。

```sh
git log --reverse --format='%h %s' v0.2.25..release/0.2.25
git log --format='%h %s' --grep='^Downstream-Patch: FAST-001$' v0.2.25..release/0.2.25
```

后续同步仍按功能判断官方覆盖情况；未发布的本地修正并入对应功能，已发布历史保留后续修复记录。

## 补丁清单

| 编号 | 功能 | 处理 | 迁移单元与来源 |
| --- | --- | --- | --- |
| BILL-001 | OpenAI 按最终出站 `service_tier` 计费 | 保留。响应档位作为诊断信息 | `9f1bf652b` |
| BILL-002 | 分组逐模型 Token 倍率 | 保留倍率优先级及已有配置，兼容官方模型广场的复制模型 ID | `77b0f99d8` |
| FAST-001 | 账号级强制与免费 Fast / Ultra Fast | 保留。账号与分组共用 Ultra Fast 资格判断，符合资格的强制 Ultra Fast 优先，最终档位继续经过全局策略 | `feb359369`、`32339fac5` |
| CTX-001 | Codex Desktop Page、浏览器上下文和 HTTP 桥接 | 将官方新增的上下文类型并入 HTTP 与 WS 转 HTTP 的共享处理，保留 `browser_tabs` | `c248ee9fc` |
| CI-001 | 固定版本 hotfix 镜像及产物版本验证 | 保留上下文、Fast 与缓存计费回归 | `afdea1301`、`1cd7a12ac` |
| TEST-001 | Fast、Ultra Fast 与缓存写入回归 | 随 FAST-001 迁移，覆盖账号档位优先级、GPT-6.2 套餐资格和 30m / 12h 用量 | `feb359369`、`32339fac5` |
| UPD-001 | 自有在线升级、远程回退与 zy 修订排序 | 保留 `zhiylee/sub2api` 的正式 zy 发行规则、独立缓存和发布适配 | `c72f69c40` |
| UPD-002 | 独立展示官方发布与新版提示 | 保留官方版本查询、独立缓存和前端提示 | `c72f69c40` |
| BUILD-001 | Go 1.27.4 工具链 | 自 `v0.2.23` 起由官方覆盖 | `1786061f7` |

HTTP Responses 与 WS 转 HTTP 每轮重放共用官方预筛选和 `backend/internal/pkg/codexcontext`
中的归一化。官方新增的 `ide_context`、`untrusted_content` 和 `proposed_plan` 沿用各自的
文本校验和格式。Page 的空 `call_id`、CRLF 分片头和单片快照沿用官方格式。
二开的 `agent.browser_tabs` 要求无 `call_id` 且匹配 ambient envelope。

Fast 配置存储在 `accounts.extra`。免费 Fast 和免费 Ultra Fast 复用 30m / 12h 用量桶，
以同一份 token 和定价重算 Standard 客户扣费，保留实际档位和对应的总费用。
账号设置、分组设置和全局策略的优先关系见
[账号级 Fast 说明](backend/dev-docs/openai-account-fast.md)。

BILL-002 把 `group_rate_multiplier` 存在现有分组 `model_pricing` 价卡。按客户端模型名匹配，
用户专属倍率优先，随后是模型倍率和分组倍率，Token 计费再叠加订阅高峰倍率。
只有倍率的价卡沿用现有单价来源。主要规则在 `backend/internal/service/group_model_rate.go`，
使用现有用量日志倍率列与分组接口。迁移或退役时一并核对认证缓存、利润门、在途余额预留、
WebSocket 逐轮刷新、密钥选组选项和模型广场。

UPD-001 / UPD-002 参考历史 `7af4074ea`、`79824dd94`、`2db65f778` 的行为重新收窄，
未整批恢复旧实现。官方的下载、校验、替换、操作锁和重启流程继续保留。
自有发行规则位于 `backend/internal/pkg/distribution`；官方查询、接口、缓存方法和前端提示
分别放在独立文件中。两项功能及其发布适配归为一个功能提交，后续可按这些文件边界部分退役。
官方支持可配置更新源、zy 修订或官方发布提醒时，逐项检查并退役相应适配，不能只替换仓库地址。
完整自有发布使用 `vX.Y.Z-zy.N`；当前 simple 模式缺少安装包，禁止用于正式自有 zy 发布。
Release 页的一键安装命令从对应版本 tag 获取自有安装脚本，避免使用仍镜像官方代码的 `main`。

## 下次升级

1. 确认官方正式 tag 和 commit，从该 tag 创建新的 `release/<版本>`。
2. 对照本表及后续修复，逐项检查官方是否完整或部分实现相同行为。
3. 对仍需保留的补丁按依赖顺序使用 `git cherry-pick -x`；部分重叠的补丁重新收窄。
4. 更新本表的来源、功能标识、VERSION 和集成说明；保留已发布分支、tag 与原工作区改动。
5. 限制本地并发，运行受影响的回归。发布前由 CI 完成全量测试和构建。

功能退役时必须连同其后续修复、消费者、已有配置和数据兼容性一起检查。

## 本地验证

在仓库根目录执行，Go 使用 `backend/go.mod` 对应的工具链。

```sh
GOMAXPROCS=1 GOMEMLIMIT=768MiB go -C backend test -p=1 -parallel=1 -tags=unit -count=1 \
  -run '^Test(Normalize(PageContext|Codex)|Codex(PageContext|CallOutputGate)|PrepareOpenAIWSHTTPBridgeBody|OpenAIAccountFast|EvaluateOpenAIFastPolicy|ApplyOpenAIFastPolicyTo|GPT62|ResolveBillingServiceTier|ResolveOpenAIServiceTierBilling|ApplyServiceTierBillingResolution|OpenAICacheWrite|OpenAIGatewayServiceRecordUsage_|GroupModelRate)' \
  ./internal/pkg/codexcontext ./internal/service ./internal/handler
pnpm --dir frontend exec vitest run --maxWorkers=1 --minWorkers=1 --no-file-parallelism \
  src/components/account/__tests__/EditAccountModal.spec.ts \
  src/components/admin/channel/__tests__/PricingEntryCard.modelDefaultPrice.spec.ts \
  src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts \
  src/views/user/__tests__/KeysView.spec.ts \
  src/i18n/__tests__/localeKeyCompleteness.spec.ts
NODE_OPTIONS=--max-old-space-size=2048 pnpm --dir frontend run typecheck
git diff --check
```

以下命令检查在线升级与发布适配。

```sh
GOMAXPROCS=1 GOMEMLIMIT=1GiB go -C backend test -p=1 -parallel=1 -tags=unit -count=1 \
  -run '^Test(CompareVersions|ReleaseTags|UpdateService|UpdateCacheReleaseChannels|SystemHandler)' \
  ./internal/pkg/distribution ./internal/service ./internal/repository ./internal/handler/admin
NODE_OPTIONS=--max-old-space-size=768 pnpm --dir frontend exec vitest run \
  --maxWorkers=1 --minWorkers=1 --no-file-parallelism \
  src/components/common/__tests__/VersionBadge.spec.ts \
  src/composables/__tests__/useUpstreamRelease.spec.ts \
  src/stores/__tests__/app.version.spec.ts \
  src/stores/__tests__/app.spec.ts \
  src/api/__tests__/admin.system.rollback.spec.ts \
  src/i18n/__tests__/localeKeyCompleteness.spec.ts
python3 -m unittest discover -s .github/release-tools -p 'test_*release*.py'
bash deploy/tests/install-downstream-test.sh
bash deploy/tests/install-github-token-test.sh
```

在线升级前端回归已接入 `make test-frontend-critical`，发布和安装脚本回归已接入 CI。

升级配置及验证结果见 [v0.2.25-zy.1 集成说明](deploy/releases/v0.2.25-zy.1.md)和 [v0.2.25-zy.2 发布说明](deploy/releases/v0.2.25-zy.2.md)。
