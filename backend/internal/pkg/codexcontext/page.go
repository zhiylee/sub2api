// Package codexcontext normalizes ambient Codex Desktop context for HTTP Responses.
package codexcontext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var codexSelectedPageName = regexp.MustCompile(`^codex_apps_selected_page_part_([1-9][0-9]*)_of_([1-9][0-9]*)$`)

// NormalizePageContext adapts standalone Page and browser snapshots to untrusted
// user text. HTTP handlers and WS-to-HTTP replay must share this normalization.
// Native WebSocket forwarding keeps its original input.
func NormalizePageContext(body []byte) ([]byte, bool) {
	if !ContainsCallOutputCandidate(body, "codex_apps_open_page", "codex_apps_selected_page_part_", "browser_tabs") || !HasUniqueJSONMembers(body) {
		return body, false
	}
	var request map[string]json.RawMessage
	if json.Unmarshal(body, &request) != nil {
		return body, false
	}
	var input []json.RawMessage
	if json.Unmarshal(request["input"], &input) != nil {
		return body, false
	}
	changed := false
	for i, raw := range input {
		var item map[string]json.RawMessage
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		readString := func(key string) string {
			var value string
			_ = json.Unmarshal(item[key], &value)
			return value
		}
		if readString("type") != "function_call_output" || readString("namespace") != "agent" {
			continue
		}
		if callID, exists := item["call_id"]; exists {
			var value string
			if bytes.Equal(bytes.TrimSpace(callID), []byte("null")) || json.Unmarshal(callID, &value) != nil || strings.TrimSpace(value) != "" {
				continue
			}
		}
		name, output := readString("name"), readString("output")
		// Browser context is ambient only when no call_id member is present.
		if _, hasCallID := item["call_id"]; name == "browser_tabs" && hasCallID {
			continue
		}
		if !validCodexPageContext(name, output) {
			continue
		}
		message := map[string]any{
			"type": "message",
			"role": "user",
			"content": []map[string]string{{
				"type": "input_text",
				"text": "<external_" + name + ">\n" + output + "\n</external_" + name + ">",
			}},
		}
		encoded, err := json.Marshal(message)
		if err != nil {
			return body, false
		}
		input[i] = encoded
		changed = true
	}
	if !changed {
		return body, false
	}
	encodedInput, err := json.Marshal(input)
	if err != nil {
		return body, false
	}
	request["input"] = encodedInput
	encoded, err := json.Marshal(request)
	if err != nil {
		return body, false
	}
	return encoded, true
}

func validCodexPageContext(name, output string) bool {
	if name == "browser_tabs" {
		// Browser text is opaque; recognize only the ambient envelope.
		context, ok := strings.CutPrefix(strings.TrimSpace(output), `<in-app-browser-context source="ambient-ui-state">`)
		if !ok {
			return false
		}
		context, ok = strings.CutSuffix(context, "</in-app-browser-context>")
		return ok && strings.TrimSpace(context) != ""
	}
	if name == "codex_apps_open_page" {
		return validCodexPageSnapshot(output)
	}
	parts := codexSelectedPageName.FindStringSubmatch(name)
	if len(parts) != 3 {
		return false
	}
	part, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	total, err := strconv.Atoi(parts[2])
	if err != nil || part > total {
		return false
	}
	header, snapshot, ok := strings.Cut(output, "\n")
	if !ok || strings.TrimSuffix(header, "\r") != fmt.Sprintf("Companion Page snapshot part %d of %d:", part, total) || strings.TrimSpace(snapshot) == "" {
		return false
	}
	// A multi-part snapshot may split inside a JSON string; preserve each chunk verbatim.
	return total > 1 || validCodexPageSnapshot(snapshot)
}

func validCodexPageSnapshot(text string) bool {
	if !HasUniqueJSONMembers([]byte(text)) {
		return false
	}
	var snapshot map[string]json.RawMessage
	if json.Unmarshal([]byte(text), &snapshot) != nil {
		return false
	}
	id, exists := snapshot["page_id"]
	if !exists {
		return false
	}
	if bytes.Equal(bytes.TrimSpace(id), []byte("null")) {
		return true
	}
	var value string
	return json.Unmarshal(id, &value) == nil && strings.TrimSpace(value) != ""
}

// ContainsCallOutputCandidate avoids decoding requests without a matching output.
func ContainsCallOutputCandidate(body []byte, toolNames ...string) bool {
	if bytes.Contains(body, []byte(`\u`)) {
		return true
	}
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

// HasUniqueJSONMembers rejects invalid JSON and duplicate keys at any depth.
func HasUniqueJSONMembers(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if !consumeUniqueJSONValue(decoder) {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

func consumeUniqueJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return true
	}

	switch delim {
	case '{':
		members := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return false
			}
			key, ok := keyToken.(string)
			if !ok {
				return false
			}
			if _, duplicate := members[key]; duplicate {
				return false
			}
			members[key] = struct{}{}
			if !consumeUniqueJSONValue(decoder) {
				return false
			}
		}
		end, err := decoder.Token()
		return err == nil && end == json.Delim('}')
	case '[':
		for decoder.More() {
			if !consumeUniqueJSONValue(decoder) {
				return false
			}
		}
		end, err := decoder.Token()
		return err == nil && end == json.Delim(']')
	default:
		return false
	}
}
