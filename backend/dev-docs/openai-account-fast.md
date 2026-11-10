# 账号级 OpenAI Fast

在「账号管理 → 编辑账号 → OpenAI Fast 模式」设置两个独立开关：

- **强制使用 Fast（priority）**：选中该账号后，把请求档位写为 `priority`，然后继续执行全局 Fast/Flex 策略。
- **免费 Fast**：对该账号最终以 `priority` 发出的请求，按相同请求的 Standard 价格向客户扣费。本开关不主动申请 Fast，也不免除上游费用。

配置存放在现有 `accounts.extra` 中，无需数据库迁移：

```json
{
  "openai_force_fast": true,
  "openai_free_fast": true
}
```

未配置时均为关闭。账号与分组设置采用 OR 关系：任一处开启即生效，关闭账号开关不会取消分组配置。全局策略仍可过滤或拦截强制写入的 `priority`。

配置仅对 OpenAI 账号生效，覆盖 OAuth、Setup Token 和 API Key 文本转发，包括 Responses、Chat Completions、Messages 转换与 WebSocket 的 `response.create`。图片、视频等独立端点不额外注入此参数。已有 WebSocket 会话需要重连。

影子账号使用自身的 Fast 设置，不自动继承母账号。故障切换时按每次选中的账号计算，原始入站请求体保持可复用。

免费 Fast 保留用量记录的 `service_tier=priority` 及 Fast 基准费用，只把客户实际扣费重算为 Standard；`flex`、`default`、`ultrafast` 等档位不适用。

## 同步上游时的检查点

账号配置读取、校验及组合判断集中在 `backend/internal/service/openai_account_fast.go`，界面集中在 `frontend/src/components/account/OpenAIAccountFastSettings.vue`。其余接入点为：

1. `admin_account.go`：创建、编辑和 extra 更新入口的布尔值校验。
2. `openai_gateway_request_body.go`、`openai_gateway_forward.go`：使用账号与分组组合后的强制 Fast 判断。
3. `openai_gateway_messages_chat_fallback.go`：转换为 Chat Completions 后应用策略，并从最终出站报文提取档位。
4. `openai_gateway_usage.go`：用实际选中的账号判断免费 Fast，继续复用原有 Standard 重算。
5. `EditAccountModal.vue`：回填开关，并在提交末尾合并配置，避免其它 extra 构建逻辑覆盖。

配置只在选号后读取；当前转发使用完整账号快照，复用普通账号编辑的刷新机制，无需扩展调度候选投影。若上游将判断提前到选号阶段，需同时检查精简投影是否保留这两个键。

## 验证

CI 包含前后端编译。后端定向回归：

```sh
cd backend
GOMAXPROCS=1 GOMEMLIMIT=512MiB go test -p=1 -tags=unit -run '^TestOpenAIAccountFast' ./internal/service
```

前端开关回填、独立保存、关闭和切换账号的测试位于已有的 `EditAccountModal.spec.ts`，由现有 critical vitest 覆盖。
