package service

import (
	"bytes"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/codexcontext"
	"github.com/tidwall/gjson"
)

// NormalizeCodexUntrustedContext uses the same output probe and normalization
// for HTTP Responses and WebSocket turns bridged to HTTP.
func NormalizeCodexUntrustedContext(body []byte) ([]byte, bool) {
	if !MayContainCodexCallOutputCandidate(body,
		"codex_apps_open_page", "codex_apps_selected_page_part_",
		"browser_info", "goal_objective", "untrusted_context", "untrusted_content", "ide_context", "proposed_plan", "browser_tabs",
	) {
		return body, false
	}
	return codexcontext.NormalizeUntrustedContext(body)
}

// MayContainCodexCallOutputCandidate uses a byte prefilter before scanning
// input for a candidate. Tool declarations alone do not trigger decoding.
func MayContainCodexCallOutputCandidate(body []byte, toolNames ...string) bool {
	if !JSONMayEscapePlainASCII(body) && !containsCodexCallOutputLiterals(body, toolNames) {
		return false
	}
	return ResponsesInputHasFunctionCallOutputNamed(body, toolNames...)
}

func containsCodexCallOutputLiterals(body []byte, toolNames []string) bool {
	if !bytes.Contains(body, []byte("function_call_output")) {
		return false
	}
	for _, name := range toolNames {
		if bytes.Contains(body, []byte(name)) {
			return true
		}
	}
	return false
}

// ResponsesInputHasFunctionCallOutputNamed reports whether the top-level input
// array holds an object item whose type is "function_call_output" and whose
// name starts with one of namePrefixes. Keys and string values are compared
// after JSON unescaping, through a zero-copy view of body. When a member is
// duplicated, the first occurrence is read; callers that decode the body
// treat duplicate members separately.
func ResponsesInputHasFunctionCallOutputNamed(body []byte, namePrefixes ...string) bool {
	input := parseRawJSONView(body).Get("input")
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if !item.IsObject() || item.Get("type").String() != "function_call_output" {
			return true
		}
		name := item.Get("name").String()
		for _, prefix := range namePrefixes {
			if strings.HasPrefix(name, prefix) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
