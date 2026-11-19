//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestGroupModelRateOnlyPreservesPricingSourceAndAuthSnapshot(t *testing.T) {
	ctx := context.Background()
	apiKey := profitAuthTestAPIKey()
	rate, price := 0.2, 2e-6
	rateCard := ChannelModelPricing{Models: []string{" gpt-5.1 "}, GroupRateMultiplier: &rate}
	wildcardPrice := ChannelModelPricing{Models: []string{"gpt-*"}, InputPrice: &price}
	pricing, err := normalizeGroupModelPricing(PlatformOpenAI, []ChannelModelPricing{rateCard, wildcardPrice})
	require.NoError(t, err)
	apiKey.Group.ModelPricing = pricing

	auth := &APIKeyService{}
	data, err := json.Marshal(APIKeyAuthCacheEntry{Snapshot: auth.snapshotFromAPIKey(ctx, apiKey)})
	require.NoError(t, err)
	var entry APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(data, &entry))
	restored, ok, err := auth.applyAuthCacheEntry(apiKey.Key, &entry)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []GroupModelRate{{Model: "gpt-5.1", Multiplier: rate}}, restored.Group.ModelRates())
	require.Nil(t, restored.Group.modelRateMultiplier("gpt-5.1-codex"))

	resolver := NewModelPricingResolver(nil, NewBillingService(nil, nil))
	resolved := resolver.Resolve(ctx, PricingInput{Model: "gpt-5.1", Group: restored.Group})
	require.Equal(t, PricingSourceGroup, resolved.Source)
	require.Equal(t, price, resolved.BasePricing.InputPricePerToken)
	restored.Group.ModelPricing = pricing[:1]
	resolved = resolver.Resolve(ctx, PricingInput{Model: "gpt-5.1", Group: restored.Group})
	require.NotEqual(t, PricingSourceGroup, resolved.Source)
	require.NotNil(t, resolved.BasePricing)

	for _, invalid := range []ChannelModelPricing{
		{Models: []string{"gpt-*"}, GroupRateMultiplier: &rate},
		{Models: []string{"gpt-5.1"}, BillingMode: BillingModePerRequest, GroupRateMultiplier: &rate},
		{Models: []string{"gpt-5.1"}, GroupRateMultiplier: new(float64)},
		{Models: []string{"gpt-5.1", "GPT-5.1"}, GroupRateMultiplier: &rate},
	} {
		_, err := normalizeGroupModelPricing(PlatformOpenAI, []ChannelModelPricing{invalid})
		require.Error(t, err)
	}
}

func TestGroupModelRateCacheKeepsRequestFallbackAndUserOverride(t *testing.T) {
	group := &Group{ID: 9, RateMultiplier: 0.3, ModelPricing: []ChannelModelPricing{
		{Models: []string{"discount"}, GroupRateMultiplier: floatPtr(0.2)},
		{Models: []string{"premium"}, GroupRateMultiplier: floatPtr(0.4)},
	}}
	repo := &userGroupRateResolverRepoStub{}
	resolver := newUserGroupRateResolver(repo, nil, time.Minute, nil, "")
	ctx := context.Background()
	for model, want := range map[string]float64{"discount": 0.2, "premium": 0.4, "other": 0.3} {
		base, rate := resolver.resolveGroupModelRates(ctx, group, group.ID, 1, model)
		require.Equal(t, want, tokenRateOr(rate, base))
	}
	require.Equal(t, 1, repo.calls)

	// An explicit user rate equal to the group rate still takes precedence.
	repo.rate = &group.RateMultiplier
	base, rate := resolver.resolveGroupModelRates(ctx, group, group.ID, 2, "discount")
	require.Equal(t, group.RateMultiplier, base)
	require.Nil(t, rate)
}

func TestGroupModelRateMatchesSettlementQuoteAndProfitControl(t *testing.T) {
	ctx := context.Background()
	const (
		alias      = "public-alias"
		upstream   = "gpt-5.1"
		targetRate = 0.4
	)
	group := profitControlTestGroup(23, 0.1, 0)
	group.RateMultiplier = 0.8
	group.SubscriptionType = SubscriptionTypeSubscription
	group.PeakRateEnabled = true
	group.PeakStart, group.PeakEnd, group.PeakRateMultiplier = "00:00", "01:00", 2
	group.ModelPricing = []ChannelModelPricing{
		{Models: []string{alias}, GroupRateMultiplier: floatPtr(0.2)},
		{Models: []string{upstream}, GroupRateMultiplier: floatPtr(0.3)},
	}
	pricingAt := time.Date(2026, time.January, 1, 0, 30, 0, 0, time.UTC)
	user := &User{ID: 24}
	key := &APIKey{ID: 25, GroupID: &group.ID, Group: group, User: user, Quota: 1}
	account := &Account{ID: 26, Type: AccountTypeAPIKey}
	sub := &UserSubscription{ID: 27}
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	billing := &openAIRecordUsageBillingRepoStub{}
	fields := ChannelUsageFields{OriginalModel: alias, ChannelMappedModel: upstream, BillingModelSource: BillingModelSourceChannelMapped}
	input := &RecordUsageInput{
		Result: &ForwardResult{Model: upstream, RequestID: "gateway-usage", Usage: ClaudeUsage{InputTokens: 1000}},
		User:   user, APIKey: key, Account: account, Subscription: sub, PricingAt: pricingAt, ChannelUsageFields: fields,
	}
	gateway := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
	gateway.resolver = NewModelPricingResolver(nil, gateway.billingService)
	require.NoError(t, gateway.RecordUsage(ctx, input))
	require.Greater(t, logs.lastLog.TotalCost, 0.0)
	require.InDelta(t, targetRate, logs.lastLog.RateMultiplier, 1e-12)
	require.InDelta(t, logs.lastLog.TotalCost*targetRate, billing.lastCmd.SubscriptionCost, 1e-12)
	require.InDelta(t, logs.lastLog.ActualCost, gateway.QuoteUsageCost(ctx, input), 1e-12)

	openAI := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil, nil)
	openAI.resolver = NewModelPricingResolver(nil, openAI.billingService)
	require.NoError(t, openAI.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{Model: upstream, RequestID: "openai-usage", Usage: OpenAIUsage{InputTokens: 1000}, OpenAIWSMode: true},
		User:   user, APIKey: key, Account: account, Subscription: sub, PricingAt: pricingAt, ChannelUsageFields: fields,
	}))
	require.Greater(t, logs.lastLog.TotalCost, 0.0)
	require.InDelta(t, targetRate, logs.lastLog.RateMultiplier, 1e-12)
	require.InDelta(t, logs.lastLog.TotalCost*targetRate, billing.lastCmd.SubscriptionCost, 1e-12)

	// The account follows the requested alias across both admission paths.
	ctx = context.WithValue(ctx, ctxkey.Group, group)
	ctx = context.WithValue(ctx, ctxkey.Model, alias)
	ctx = context.WithValue(ctx, openAIPricingAtCtxKey{}, pricingAt)
	ctx = context.WithValue(ctx, gatewayTokenRequestPricingAtCtxKey{}, pricingAt)
	gate := openAI.resolveOpenAIProfitControlGate(ctx, &group.ID)
	require.NotNil(t, gate)
	require.InDelta(t, targetRate*0.9, gate.threshold, 1e-12)
	commonCtx := gateway.withGatewayProfitControlGate(ctx, &group.ID)
	require.InDelta(t, gate.threshold, commonCtx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate).threshold, 1e-12)

	// A subsequent turn's different model is priced independently of the connection.
	ctx = context.WithValue(ctx, ctxkey.Model, upstream)
	require.InDelta(t, 0.6*0.9, openAI.resolveOpenAIProfitControlGate(ctx, &group.ID).threshold, 1e-12)
}

func TestGroupModelRateLeavesPerRequestBillingUnchanged(t *testing.T) {
	price, rate := 0.1, 0.2
	g := &Group{ID: 28, RateMultiplier: 0.6, ModelPricing: []ChannelModelPricing{
		{Models: []string{"tool"}, GroupRateMultiplier: &rate},
		{Models: []string{"tool"}, BillingMode: BillingModePerRequest, PerRequestPrice: &price},
	}}
	billing := NewBillingService(nil, nil)
	resolver := NewModelPricingResolver(nil, billing)
	ctx := withModelTokenRate(context.Background(), &rate, g, time.Now())
	cost, err := billing.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx: ctx, Model: "tool", Group: g, Resolver: resolver,
		RateMultiplier: g.RateMultiplier, modelTokenRate: modelTokenRateFromContext(ctx),
	})
	require.NoError(t, err)
	require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
	require.InDelta(t, price*g.RateMultiplier, cost.ActualCost, 1e-12)
	require.Equal(t, g.RateMultiplier, usageRateMultiplier(cost, g.RateMultiplier))
	require.Empty(t, g.ModelRates())
}
