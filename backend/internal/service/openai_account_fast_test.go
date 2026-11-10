//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIAccountFastPolicyHTTPAndWS(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken, AccountTypeAPIKey} {
		for _, transport := range []string{"http", "ws"} {
			t.Run(accountType+"/"+transport, func(t *testing.T) {
				account := &Account{Platform: PlatformOpenAI, Type: accountType, Extra: map[string]any{OpenAIForceFastExtraKey: true}}
				svc := newOpenAIGatewayServiceWithSettings(t, DefaultOpenAIFastPolicySettings())
				for _, tier := range []string{"", "default", "auto", "flex", "fast", "priority", "ultrafast"} {
					body := `{"type":"response.create","model":"gpt-5.6-sol","input":[]`
					if tier != "" {
						body += `,"service_tier":"` + tier + `"`
					}
					body += `}`
					var updated []byte
					var err error
					if transport == "http" {
						updated, err = svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.6-sol", []byte(body))
					} else {
						var blocked *OpenAIFastBlockedError
						updated, blocked, err = svc.applyOpenAIFastPolicyToWSResponseCreate(context.Background(), account, "gpt-5.6-sol", []byte(body))
						require.Nil(t, blocked)
					}
					require.NoError(t, err)
					require.Equal(t, "priority", gjson.GetBytes(updated, "service_tier").String(), "client tier: %q", tier)
				}

				for _, action := range []string{BetaPolicyActionFilter, BetaPolicyActionBlock} {
					svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
						ServiceTier: OpenAIFastTierPriority, Scope: BetaPolicyScopeAll, Action: action,
					}}})
					body := []byte(`{"type":"response.create","model":"gpt-5.6-sol","input":[]}`)
					var updated []byte
					var blocked *OpenAIFastBlockedError
					if transport == "http" {
						var err error
						updated, err = svc.applyOpenAIFastPolicyToBody(context.Background(), account, "gpt-5.6-sol", body)
						if action == BetaPolicyActionBlock {
							require.ErrorAs(t, err, &blocked)
						} else {
							require.NoError(t, err)
						}
					} else {
						var err error
						updated, blocked, err = svc.applyOpenAIFastPolicyToWSResponseCreate(context.Background(), account, "gpt-5.6-sol", body)
						require.NoError(t, err)
					}
					if action == BetaPolicyActionBlock {
						require.NotNil(t, blocked)
					} else {
						require.False(t, gjson.GetBytes(updated, "service_tier").Exists())
					}
				}
			})
		}
	}
}

func TestOpenAIAccountFastForwardAttemptsKeepClientBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Reuse the same ingress body as the handler does on account failover.
	body := []byte(`{"model":"gpt-5.6-sol","stream":true,"input":"hi"}`)
	original := bytes.Clone(body)
	for _, passthrough := range []bool{false, true} {
		for _, force := range []bool{true, false} {
			svc := newOpenAIGatewayServiceWithSettings(t, DefaultOpenAIFastPolicySettings())
			svc.cfg = &config.Config{}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
			}}
			svc.httpUpstream = upstream
			account := &Account{
				ID: 123, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
				Credentials: map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
				Extra: map[string]any{"openai_passthrough": passthrough, OpenAIForceFastExtraKey: force,
					"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
				Status: StatusActive, Schedulable: true, RateMultiplier: f64p(1),
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.144.1")
			result, err := svc.Forward(context.Background(), c, account, body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, force, gjson.GetBytes(upstream.lastBody, "service_tier").Exists())
			if force {
				require.NotNil(t, result.ServiceTier)
				require.Equal(t, "priority", *result.ServiceTier)
			} else {
				require.Nil(t, result.ServiceTier)
			}
			require.Equal(t, original, body, "account-specific rewrites must not mutate ingress bytes")
		}
	}
}

func TestOpenAIAccountFastWSLeavesOtherFramesAndAccountsAlone(t *testing.T) {
	svc := newOpenAIGatewayServiceWithSettings(t, DefaultOpenAIFastPolicySettings())
	for _, platform := range []string{PlatformOpenAI, PlatformGrok} {
		for _, force := range []bool{true, false} {
			account := &Account{Platform: platform, Extra: map[string]any{OpenAIForceFastExtraKey: force}}
			for _, frameType := range []string{"response.create", "response.cancel", "session.update"} {
				frame := []byte(`{"type":"` + frameType + `","model":"gpt-5.6-sol"}`)
				updated, blocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(context.Background(), account, "gpt-5.6-sol", frame)
				require.NoError(t, err)
				require.Nil(t, blocked)
				wantFast := platform == PlatformOpenAI && force && frameType == "response.create"
				require.Equal(t, wantFast, gjson.GetBytes(updated, "service_tier").Exists())
			}
		}
	}
}

func TestOpenAIAccountFastGroupSettingsStillApply(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{OpenAIForceFastExtraKey: false, OpenAIFreeFastExtraKey: false}}
	group := &Group{ID: 7, Platform: PlatformComposite, Status: StatusActive, Hydrated: true, ForceOpenAIFast: true, FreeOpenAIFast: true}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	require.True(t, openAIRequestForcesFast(ctx, account))
	require.True(t, billsOpenAIFastAtStandard(&APIKey{Group: group}, account, "priority"))
	for _, tier := range []string{"", "default", "auto", "flex", "ultrafast"} {
		require.False(t, billsOpenAIFastAtStandard(&APIKey{Group: group}, account, tier))
	}
	require.False(t, openAIRequestForcesFast(ctx, &Account{Platform: PlatformGrok, Extra: account.Extra}))
	account.Extra[OpenAIFreeFastExtraKey] = true
	require.False(t, openAIRequestForcesFast(context.Background(), account), "Free Fast must not request priority by itself")
	require.True(t, billsOpenAIFastAtStandard(nil, account, "priority"))
	require.False(t, billsOpenAIFastAtStandard(nil, account, "ultrafast"))
}

func TestOpenAIAccountFastMessagesChatFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, action := range []string{BetaPolicyActionPass, BetaPolicyActionFilter, BetaPolicyActionBlock} {
		t.Run(action, func(t *testing.T) {
			account := forceChatMessagesFallbackAccount()
			account.Extra[OpenAIForceFastExtraKey] = true
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{"id":"chatcmpl_fast","model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`)),
			}}
			svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
				ServiceTier: OpenAIFastTierPriority, Scope: BetaPolicyScopeAll, Action: action,
			}}})
			svc.cfg, svc.httpUpstream = rawChatCompletionsTestConfig(), upstream
			body := []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
			result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
			if action == BetaPolicyActionBlock {
				var blocked *OpenAIFastBlockedError
				require.ErrorAs(t, err, &blocked)
				require.Equal(t, http.StatusForbidden, rec.Code)
				require.Nil(t, upstream.lastReq)
				return
			}
			require.NoError(t, err)
			if action == BetaPolicyActionPass {
				require.Equal(t, "priority", gjson.GetBytes(upstream.lastBody, "service_tier").String())
				require.Equal(t, "priority", *result.ServiceTier)
			} else {
				require.False(t, gjson.GetBytes(upstream.lastBody, "service_tier").Exists())
				require.Nil(t, result.ServiceTier)
			}
		})
	}
}

func TestOpenAIAccountFastUsageBillsSelectedAccount(t *testing.T) {
	for _, tt := range []struct {
		name       string
		free       bool
		shadow     bool
		parentFree bool
	}{
		{name: "account free", free: true},
		{name: "account paid"},
		{name: "shadow free", free: true, shadow: true},
		{name: "shadow does not inherit parent", shadow: true, parentFree: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			svc.resolver = NewModelPricingResolver(nil, svc.billingService)
			inputPrice, outputPrice, fastMultiplier := 0.001, 0.002, 3.0
			groupID := int64(77)
			apiKey := &APIKey{ID: 1020, GroupID: &groupID, Group: &Group{
				ID: groupID, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RateMultiplier: 0.5,
				ModelPricing: []ChannelModelPricing{{Models: []string{"gpt-5.6-sol"}, BillingMode: BillingModeToken,
					InputPrice: &inputPrice, OutputPrice: &outputPrice, FastMultiplier: &fastMultiplier}},
			}}
			account := &Account{ID: 3020, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
				Extra: map[string]any{OpenAIFreeFastExtraKey: tt.free}}
			if tt.shadow {
				parentID := int64(9001)
				account.ParentAccountID = &parentID
				svc.accountRepo = &openAIRecordUsageAccountRepoStub{account: &Account{
					ID: parentID, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
					Extra: map[string]any{OpenAIFreeFastExtraKey: tt.parentFree},
				}}
			}
			tier := "priority"
			err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{RequestID: "resp_account_fast", Model: "gpt-5.6-sol", ServiceTier: &tier,
					UpstreamResponseServiceTier: "default", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}, Duration: time.Second},
				APIKey: apiKey, User: &User{ID: 2020}, Account: account,
			})
			require.NoError(t, err)
			require.NotNil(t, usageRepo.lastLog)
			require.Equal(t, "priority", *usageRepo.lastLog.ServiceTier)
			standardTotal := 100*inputPrice + 50*outputPrice
			require.InDelta(t, standardTotal*fastMultiplier, usageRepo.lastLog.TotalCost, 1e-10)
			wantCharge := standardTotal * 0.5
			if !tt.free {
				wantCharge *= fastMultiplier
			}
			require.InDelta(t, wantCharge, usageRepo.lastLog.ActualCost, 1e-10)
		})
	}
}

func TestOpenAIAccountFastValidation(t *testing.T) {
	for _, key := range []string{OpenAIForceFastExtraKey, OpenAIFreeFastExtraKey} {
		for _, value := range []any{nil, "true", 1, map[string]any{}} {
			extra := map[string]any{key: value}
			require.Error(t, ValidateOpenAIAccountFastExtra(extra))
			_, err := buildAccountForCreate(&CreateAccountInput{}, extra)
			require.Error(t, err)
			svc := &adminServiceImpl{}
			require.Error(t, svc.UpdateAccountExtra(context.Background(), 1, extra))
			_, err = svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{Extra: extra})
			require.Error(t, err)
		}
	}
	require.NoError(t, ValidateOpenAIAccountFastExtra(nil))
	require.NoError(t, ValidateOpenAIAccountFastExtra(map[string]any{OpenAIForceFastExtraKey: true, OpenAIFreeFastExtraKey: false}))
}

func TestOpenAIAccountFastAdminUpdate(t *testing.T) {
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
		1: {ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive},
	}}
	svc := &adminServiceImpl{accountRepo: repo}
	ctx := context.Background()
	for _, enabled := range []bool{true, false} {
		updated, err := svc.UpdateAccount(ctx, 1, &UpdateAccountInput{Extra: map[string]any{
			OpenAIForceFastExtraKey: enabled, OpenAIFreeFastExtraKey: enabled, "custom": "keep",
		}})
		require.NoError(t, err)
		require.Equal(t, enabled, updated.Extra[OpenAIForceFastExtraKey])
		require.Equal(t, enabled, updated.Extra[OpenAIFreeFastExtraKey])
		require.Equal(t, "keep", updated.Extra["custom"])
	}
	_, err := svc.UpdateAccount(ctx, 1, &UpdateAccountInput{Extra: map[string]any{OpenAIForceFastExtraKey: "true"}})
	require.Error(t, err)
	require.Equal(t, false, repo.accounts[1].Extra[OpenAIForceFastExtraKey])
}
