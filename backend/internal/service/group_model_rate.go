package service

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// GroupModelRate is the public subset of a group pricing card.
type GroupModelRate struct {
	Model      string  `json:"model"`
	Multiplier float64 `json:"multiplier"`
}

func validateGroupModelRates(pricing []ChannelModelPricing) error {
	models := make(map[string]struct{})
	for _, entry := range pricing {
		if entry.GroupRateMultiplier == nil {
			continue
		}
		rate := *entry.GroupRateMultiplier
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			return infraerrors.BadRequest("INVALID_GROUP_MODEL_RATE", "group model rate must be a finite number greater than zero")
		}
		if !isTokenBillingMode(entry.BillingMode) {
			return infraerrors.BadRequest("INVALID_GROUP_MODEL_RATE_MODE", "group model rate requires token billing")
		}
		for _, name := range entry.Models {
			model := normalizeChannelPricingModelName(name)
			if model == "" || strings.Contains(model, "*") {
				return infraerrors.BadRequest("INVALID_GROUP_MODEL_RATE_MODEL", "group model rate requires an exact model name")
			}
			if _, exists := models[model]; exists {
				return infraerrors.BadRequest("DUPLICATE_GROUP_MODEL_RATE", "duplicate group model rate for "+name)
			}
			models[model] = struct{}{}
		}
	}
	return nil
}

func isTokenBillingMode(mode BillingMode) bool {
	return mode == "" || mode == BillingModeToken
}

// A rate-only card must leave the price source and wildcard price cards intact.
func groupModelRateOnly(entry *ChannelModelPricing) bool {
	return entry.GroupRateMultiplier != nil && isTokenBillingMode(entry.BillingMode) &&
		entry.InputPrice == nil && entry.OutputPrice == nil &&
		entry.CacheWritePrice == nil && entry.CacheWrite1hPrice == nil && entry.CacheWrite12hPrice == nil &&
		entry.CacheReadPrice == nil && entry.ImageInputPrice == nil && entry.ImageOutputPrice == nil &&
		entry.FastMultiplier == nil && entry.FlexMultiplier == nil && entry.UltrafastMultiplier == nil &&
		entry.PerRequestPrice == nil && len(entry.Intervals) == 0 && len(entry.ReasoningEffortMultipliers) == 0
}

func groupUnitPricingEntries(pricing []ChannelModelPricing) []ChannelModelPricing {
	var entries []ChannelModelPricing
	for i := range pricing {
		if !groupModelRateOnly(&pricing[i]) {
			entries = append(entries, pricing[i])
		}
	}
	return entries
}

func (g *Group) modelRateMultiplier(model string) *float64 {
	if g == nil || len(g.ModelPricing) == 0 {
		return nil
	}
	model = normalizeChannelPricingModelName(model)
	if model == "" {
		return nil
	}
	for _, entry := range g.ModelPricing {
		if entry.GroupRateMultiplier == nil || !isTokenBillingMode(entry.BillingMode) {
			continue
		}
		for _, name := range entry.Models {
			if normalizeChannelPricingModelName(name) == model {
				return entry.GroupRateMultiplier
			}
		}
	}
	return nil
}

func (g *Group) ModelRates() []GroupModelRate {
	var rates []GroupModelRate
	for _, entry := range g.ModelPricing {
		if entry.GroupRateMultiplier == nil || !isTokenBillingMode(entry.BillingMode) {
			continue
		}
		for _, model := range entry.Models {
			if pricing := matchGroupModelPricing(g, model); pricing != nil && !isTokenBillingMode(pricing.BillingMode) {
				continue
			}
			rates = append(rates, GroupModelRate{Model: model, Multiplier: *entry.GroupRateMultiplier})
		}
	}
	return rates
}

func cloneGroupPricingWithModelRates(pricing []ChannelModelPricing) []ChannelModelPricing {
	var cloned []ChannelModelPricing
	for _, entry := range pricing {
		if entry.GroupRateMultiplier != nil {
			cloned = append(cloned, entry.Clone())
		}
	}
	return cloned
}

func (r *userGroupRateResolver) resolveGroupModelRates(ctx context.Context, group *Group, groupID, userID int64, model string) (base float64, modelRate *float64) {
	if override, ok := r.resolveOverride(ctx, userID, groupID); ok {
		return override, nil
	}
	return group.RateMultiplier, group.modelRateMultiplier(model)
}

type modelTokenRateCtxKey struct{}
type groupModelRateExemptCtxKey struct{}

// WithoutGroupModelRate marks an endpoint whose model name identifies a tool
// but whose usage is always billed per call.
func WithoutGroupModelRate(ctx context.Context) context.Context {
	return context.WithValue(ctx, groupModelRateExemptCtxKey{}, struct{}{})
}

func groupRateRequestModel(ctx context.Context, fallback string) string {
	if _, exempt := ctx.Value(groupModelRateExemptCtxKey{}).(struct{}); exempt {
		return ""
	}
	if model, ok := RequestedPublicModelFromContext(ctx); ok {
		return model
	}
	return fallback
}

func withModelTokenRate(ctx context.Context, rate *float64, group *Group, pricingAt time.Time) context.Context {
	if rate == nil {
		return ctx
	}
	tokenRate := *rate * group.PeakMultiplierAt(pricingAt)
	return context.WithValue(ctx, modelTokenRateCtxKey{}, &tokenRate)
}

func modelTokenRateFromContext(ctx context.Context) *float64 {
	if ctx == nil {
		return nil
	}
	rate, _ := ctx.Value(modelTokenRateCtxKey{}).(*float64)
	return rate
}

func tokenRateOr(rate *float64, fallback float64) float64 {
	if rate != nil {
		return *rate
	}
	return fallback
}

func usageRateMultiplier(cost *CostBreakdown, fallback float64) float64 {
	if cost != nil {
		return tokenRateOr(cost.modelTokenRate, fallback)
	}
	return fallback
}

// profitControlBillingRate uses the authenticated group's customer rate. Its
// admission check can resolve a configured channel price before account selection.
func profitControlBillingRate(ctx context.Context, group *Group, pricingAt time.Time, rates *userGroupRateResolver, resolver *ModelPricingResolver, channels *ChannelService) float64 {
	userID, _ := ctx.Value(ctxkey.UserID).(int64)
	model, _ := ctx.Value(ctxkey.Model).(string)
	base, rate := rates.resolveGroupModelRates(ctx, group, group.ID, userID, groupRateRequestModel(ctx, model))
	if rate != nil && groupModelUsesTokenPricing(ctx, group, model, resolver, channels) {
		base = *rate
	}
	return base * group.PeakMultiplierAt(pricingAt)
}

func groupModelUsesTokenPricing(ctx context.Context, group *Group, model string, resolver *ModelPricingResolver, channels *ChannelService) bool {
	if resolver == nil {
		return true
	}
	if upstream, ok := ResolvedUpstreamModelFromContext(ctx); ok {
		model = upstream
	}
	if channels != nil {
		mapping := channels.ResolveChannelMapping(ctx, group.ID, model)
		if mapping.BillingModelSource != BillingModelSourceRequested && mapping.MappedModel != "" {
			model = mapping.MappedModel
		}
	}
	return isTokenBillingMode(resolver.Resolve(ctx, PricingInput{Model: model, GroupID: &group.ID, Group: group}).Mode)
}
