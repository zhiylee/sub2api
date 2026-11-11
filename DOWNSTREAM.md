# 二开维护与社区同步

## 二开基本原则

以尽量小的侵入和后续同步成本实现需求。优先复用官方接口和执行流程，新增逻辑尽量放在
独立文件、组件中，在官方文件只保留必要接入点；避免复制整套实现、无关重构和大范围格式调整。
不能只按改动行数判断成本，还要检查行为、数据、缓存和发布渠道是否完整兼容。
每项功能独立记录来源、后续修复和退役条件；官方完整或部分覆盖后，及时删除或收窄补丁。

自有在线升级与官方发布信息的方案见
[升级渠道实现说明](backend/dev-docs/downstream-update-channels.md)；已接入当前分支，产物以正式 Release 为准。

## 当前基线

- 官方正式版：[`v0.2.21`](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.21)
- 官方提交：`10ba8f9b4dc2e6023d4a9879ab77b21841babd10`
- 集成分支：`release/0.2.21`
- 集成版本及发布 tag：`0.2.21-zy.1` / `v0.2.21-zy.1`。
- 迁移来源：`release/0.2.20` / `hotfix-v0.2.20-zy.2` 的 `97080e78d`，以及原工作区的浏览器上下文修复和回归测试。

## 提交组织

当前基线上的二开整理为 5 个功能提交，按依赖顺序为：出站档位计费、账号 Fast、
上下文兼容、hotfix 镜像、自有升级渠道。测试、文档和同一功能的修正随功能提交；
Fast / TTL 组合回归并入账号 Fast，hotfix 工作流的后续调整归入镜像提交，
自有升级的运行逻辑、发布和安装适配作为完整功能一并迁移。

每个功能提交使用稳定的 `Downstream-Patch` 标识，便于重排或迁移后定位，避免文档依赖旧提交哈希：

```sh
git log --reverse --format='%h %s' v0.2.21..release/0.2.21
git log --format='%h %s' --grep='^Downstream-Patch: FAST-001$' v0.2.21..release/0.2.21
```

后续同步仍按功能判断官方覆盖情况；未发布的本地修正并入对应功能，已发布历史保留后续修复记录。

## 补丁清单

| 编号 | 功能 | 处理 | 迁移单元与来源 |
| --- | --- | --- | --- |
| BILL-001 | OpenAI 按最终出站 `service_tier` 计费 | 保留；官方仍允许 API Key 等路径根据响应档位降低计费，与二开口径不同 | 独立功能提交；来源 `4d178adca` |
| FAST-001 | 账号级强制 Fast / 免费 Fast | 保留；官方现有功能为分组级开关，未包含账号级设置 | 含说明及 TEST-001；来源 `f9114892f`、`97080e78d` |
| CTX-001 | Codex Desktop Page、浏览器上下文和 HTTP 桥接 | 部分退役；HTTP Page 使用官方实现，二开仅补浏览器上下文和 WS 转 HTTP 覆盖 | 独立功能提交；替代 `c37fae89a` 并纳入原工作区浏览器修复 |
| CI-001 | 固定版本 hotfix 镜像及产物版本验证 | 保留，新增公共上下文包、Fast 与 TTL 计费回归 | 含全部工作流调整；来源 `8273b511e`；依赖 BILL-001、FAST-001、CTX-001 |
| TEST-001 | Fast 与新版缓存写入 TTL 的组合回归 | 新增；同时验证 OAuth / API Key、收费 / 免费 Fast、30m / 12h 用量和实际扣费 | 已并入 FAST-001，不再单独迁移 |
| UPD-001 | 自有在线升级、远程回退与 zy 修订排序 | 新增；只安装 `zhiylee/sub2api` 的正式 zy 版本，隔离旧缓存，脚本和镜像命令匹配自有渠道 | 与 UPD-002 同一功能提交，包含运行逻辑、发布与安装适配 |
| UPD-002 | 独立展示官方发布与新版提示 | 新增；只读查询官方版本，对照当前基线提示待同步，独立缓存和前端组件 | 与 UPD-001 同一功能提交，复用其客户端、版本与缓存基础设施 |

官方 Page 修复为 `b2a695d61`（PR #8622）。其转换和 JSON 成员唯一性校验移入
`backend/internal/pkg/codexcontext`，原 HTTP handler 委托调用，桥接在每轮重放组装后调用同一实现。
旧的 `backend/internal/service/openai_codex_page_context.go` 不再迁入，避免双重归一化和重复维护。
Page 的空 `call_id`、CRLF 分片头和单片快照形状沿用官方行为；浏览器仅转换完全没有
`call_id` 的 `agent.browser_tabs` ambient envelope。真实工具输出继续走原有校验。

Fast 配置继续存储在 `accounts.extra`，不增加数据库迁移。免费 Fast 复用新版的
30m / 12h 用量桶，以同一份 token 和定价重算 Standard 客户扣费，保留 Fast 总费用和档位记录。
账号设置、分组设置和全局策略的优先关系见
[账号级 Fast 说明](backend/dev-docs/openai-account-fast.md)。

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
此次范围以当前 0.2.20 集成分支为准；更早分支上的独立功能需单独核对其接纳状态。

## 本地验证

在仓库根目录执行（Go 使用 `backend/go.mod` 对应工具链）：

```sh
GOMAXPROCS=1 GOMEMLIMIT=768MiB go -C backend test -p=1 -parallel=1 -tags=unit -count=1 \
  -run '^Test(Normalize(PageContext|Codex)|CodexPageContext|PrepareOpenAIWSHTTPBridgeBody|OpenAIAccountFast|EvaluateOpenAIFastPolicy|ApplyOpenAIFastPolicyTo|ResolveBillingServiceTier|ResolveOpenAIServiceTierBilling|ApplyServiceTierBillingResolution|OpenAICacheWrite|OpenAIGatewayServiceRecordUsage_)' \
  ./internal/pkg/codexcontext ./internal/service ./internal/handler
pnpm --dir frontend exec vitest run --maxWorkers=1 --minWorkers=1 --no-file-parallelism \
  src/components/account/__tests__/EditAccountModal.spec.ts \
  src/i18n/__tests__/localeKeyCompleteness.spec.ts
NODE_OPTIONS=--max-old-space-size=2048 pnpm --dir frontend run typecheck
git diff --check
```

在线升级与发布适配的本地预检（限制并发，不启动真实更新或下载产物）：

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

新前端回归已接入 `make test-frontend-critical`，发布和安装脚本回归已接入 CI。

升级配置、数据库迁移及本次验证结果见
[v0.2.21 二开集成说明](deploy/releases/v0.2.21-zy.1.md)。
