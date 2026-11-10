package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	OpenAIForceFastExtraKey = "openai_force_fast"
	OpenAIFreeFastExtraKey  = "openai_free_fast"
)

// ValidateOpenAIAccountFastExtra validates explicitly supplied account settings.
// Missing keys keep existing behavior; only OpenAI accounts consume these flags.
func ValidateOpenAIAccountFastExtra(extra map[string]any) error {
	for _, key := range []string{OpenAIForceFastExtraKey, OpenAIFreeFastExtraKey} {
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

// Account settings add to group settings; global Fast/Flex policy still runs
// after priority is injected. Read the selected account on each forwarding attempt.
func openAIRequestForcesFast(ctx context.Context, account *Account) bool {
	return accountOpenAIFastEnabled(account, OpenAIForceFastExtraKey) || openAIGroupForcesFast(ctx, account)
}

// Free Fast only changes the customer charge for priority requests. Use the
// selected account, including shadows, rather than its credential-bearing parent.
func billsOpenAIFastAtStandard(apiKey *APIKey, account *Account, serviceTier string) bool {
	switch normalizeBillingServiceTier(serviceTier) {
	case "priority", "fast":
		return accountOpenAIFastEnabled(account, OpenAIFreeFastExtraKey) ||
			groupBillsOpenAIFastAtStandard(apiKey, account, serviceTier)
	default:
		return false
	}
}
