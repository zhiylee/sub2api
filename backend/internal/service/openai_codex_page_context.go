package service

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// NormalizeCodexPageContext adapts Codex Desktop's standalone Page inputs for
// HTTP Responses upstreams. Run it before HTTP tool-output validation or after
// WS replay assembly, without changing paired tool results or native WS input.
func NormalizeCodexPageContext(body []byte) ([]byte, bool) {
	// Escaped JSON discriminators must still reach the structural checks.
	if !bytes.Contains(body, []byte(`\u`)) &&
		(!bytes.Contains(body, []byte("function_call_output")) || !bytes.Contains(body, []byte("codex_apps_"))) {
		return body, false
	}
	if !gjson.ValidBytes(body) {
		return body, false
	}
	root := parseRawJSONView(body)
	input := root.Get("input")
	if !root.IsObject() || !input.IsArray() || hasDuplicateJSONObjectKeys(root) {
		return body, false
	}

	var items []string
	changed := false
	input.ForEach(func(_, item gjson.Result) bool {
		encoded := item.Raw
		if item.IsObject() && item.Get("type").Str == "function_call_output" &&
			item.Get("namespace").Str == "agent" && !item.Get("call_id").Exists() &&
			!hasDuplicateJSONObjectKeys(item) {
			name := item.Get("name").Str
			output := item.Get("output")
			if output.Type == gjson.String && validCodexPageContextOutput(name, output.Str) {
				// Keep the producer name as provenance, using Codex's external
				// context envelope, and preserve the output text verbatim inside it.
				message := struct {
					Type    string           `json:"type"`
					Role    string           `json:"role"`
					Content []map[string]any `json:"content"`
				}{
					Type: "message",
					Role: "user",
					Content: []map[string]any{{
						"type": "input_text",
						"text": "<external_" + name + ">" + output.Str + "</external_" + name + ">",
					}},
				}
				if normalized, err := json.Marshal(message); err == nil {
					encoded = string(normalized)
					changed = true
				}
			}
		}
		items = append(items, encoded)
		return true
	})
	if !changed {
		return body, false
	}
	// Preserve unrelated fields, numbers and input items as raw JSON. Large
	// images and tool results are copied only into the final request body.
	return replaceOpenAIRawInput(body, input, items), true
}

func validCodexPageContextOutput(name, output string) bool {
	if name == "codex_apps_open_page" {
		return validCodexPageSnapshot(output, false)
	}
	indices, ok := strings.CutPrefix(name, "codex_apps_selected_page_part_")
	if !ok {
		return false
	}
	partText, totalText, ok := strings.Cut(indices, "_of_")
	if !ok {
		return false
	}
	part, partErr := strconv.Atoi(partText)
	total, totalErr := strconv.Atoi(totalText)
	if partErr != nil || totalErr != nil || part < 1 || part > total ||
		strconv.Itoa(part) != partText || strconv.Itoa(total) != totalText {
		return false
	}
	prefix := "Companion Page snapshot part " + partText + " of " + totalText + ":\n"
	snapshot, ok := strings.CutPrefix(output, prefix)
	if !ok || strings.TrimSpace(snapshot) == "" {
		return false
	}
	// A multipart snapshot may split JSON between parts. Match its numbered
	// name and header without reassembling, reordering or parsing fragments.
	return total > 1 || validCodexPageSnapshot(snapshot, true)
}

func validCodexPageSnapshot(value string, selected bool) bool {
	if !gjson.Valid(value) {
		return false
	}
	snapshot := gjson.Parse(value)
	if !snapshot.IsObject() || hasDuplicateJSONObjectKeys(snapshot) {
		return false
	}
	pageID := snapshot.Get("page_id")
	if !pageID.Exists() || (pageID.Type != gjson.Null &&
		(pageID.Type != gjson.String || strings.TrimSpace(pageID.Str) == "")) {
		return false
	}
	// Page contents are opaque here; only the enclosing snapshot is checked.
	return !selected || snapshot.Get("document_view").Exists()
}
