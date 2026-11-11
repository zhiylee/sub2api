package handler

import "github.com/Wei-Shaw/sub2api/internal/service"

// Share Codex context normalization with HTTP bridge replay.
func normalizeCodexUntrustedContext(body []byte) ([]byte, bool) {
	return service.NormalizeCodexUntrustedContext(body)
}
