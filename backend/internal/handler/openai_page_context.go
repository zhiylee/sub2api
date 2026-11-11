package handler

import "github.com/Wei-Shaw/sub2api/internal/pkg/codexcontext"

// Share the upstream Page normalizer with HTTP bridge replay.
func normalizeCodexPageContext(body []byte) ([]byte, bool) {
	return codexcontext.NormalizePageContext(body)
}
