package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	OpenAIForceFastExtraKey     = "openai_force_fast"
	OpenAIFreeFastExtraKey      = "openai_free_fast"
	OpenAIFreeUltrafastExtraKey = "openai_free_ultrafast"
)

// ValidateOpenAIAccountFastExtra validates explicitly supplied account settings.
// Missing keys keep existing behavior; only OpenAI accounts consume these flags.
func ValidateOpenAIAccountFastExtra(extra map[string]any) error {
	for _, key := range []string{OpenAIForceFastExtraKey, OpenAIFreeFastExtraKey, OpenAIFreeUltrafastExtraKey} {
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

// openAIRequestForcedServiceTier preserves the group's model and entitlement
// checks for Ultrafast. The selected account can also force priority before
// the global policy runs.
func openAIRequestForcedServiceTier(ctx context.Context, account *Account, model string) string {
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
