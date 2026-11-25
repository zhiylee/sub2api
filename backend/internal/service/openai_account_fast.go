package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/tidwall/gjson"
)

const (
	OpenAIForceFastExtraKey      = "openai_force_fast"
	OpenAIFreeFastExtraKey       = "openai_free_fast"
	OpenAIForceUltrafastExtraKey = "openai_force_ultrafast"
	OpenAIFreeUltrafastExtraKey  = "openai_free_ultrafast"
)

// ValidateOpenAIAccountFastExtra validates explicitly supplied account settings.
// Missing keys keep existing behavior; only OpenAI accounts consume these flags.
func ValidateOpenAIAccountFastExtra(extra map[string]any) error {
	for _, key := range []string{OpenAIForceFastExtraKey, OpenAIFreeFastExtraKey, OpenAIForceUltrafastExtraKey, OpenAIFreeUltrafastExtraKey} {
		if value, exists := extra[key]; exists {
			if _, ok := value.(bool); !ok {
				return infraerrors.BadRequest("OPENAI_ACCOUNT_FAST_INVALID", key+" must be a boolean")
			}
		}
	}
	return nil
}

func accountOpenAIFastEnabled(account *Account, key string) bool {
	if account == nil || !account.IsOpenAI() {
		return false
	}
	enabled, _ := account.Extra[key].(bool)
	return enabled
}

func openAIAccountSupportsUltrafast(account *Account, model string) bool {
	if account == nil || !account.IsOpenAI() || !openAIModelSupportsUltrafast(model) {
		return false
	}
	// GPT-6.2 subscription access comes from the Pro plan or the account's model catalog.
	base := openai.GPT62ModelID(model)
	if base == "" || !account.IsOpenAIOAuthLike() || gpt62PlanIncludesUltrafast(account, base) {
		return true
	}
	metadata, ok := account.GetUpstreamModelMetadata(base)
	return ok && gjson.GetBytes(metadata.CodexToolCapabilities["service_tiers"], `#(id=="ultrafast")`).Exists()
}

// openAIRequestForcedServiceTier gives eligible account or group Ultrafast
// precedence over account or group Fast before the global policy runs.
func openAIRequestForcedServiceTier(ctx context.Context, account *Account, model string) string {
	if accountOpenAIFastEnabled(account, OpenAIForceUltrafastExtraKey) && openAIAccountSupportsUltrafast(account, model) {
		return OpenAIFastTierUltrafast
	}
	if tier := openAIGroupForcedServiceTier(ctx, account, model); tier != "" {
		return tier
	}
	if accountOpenAIFastEnabled(account, OpenAIForceFastExtraKey) {
		return OpenAIFastTierPriority
	}
	return ""
}

// Use the selected account so shadow accounts can set their own billing policy.
func billsOpenAIFastAtStandard(apiKey *APIKey, account *Account, serviceTier string) bool {
	switch normalizeBillingServiceTier(serviceTier) {
	case "priority", "fast":
		return accountOpenAIFastEnabled(account, OpenAIFreeFastExtraKey) ||
			groupBillsOpenAIFastAtStandard(apiKey, account, serviceTier)
	case OpenAIFastTierUltrafast:
		return accountOpenAIFastEnabled(account, OpenAIFreeUltrafastExtraKey)
	default:
		return false
	}
}
