package service

import (
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

// These outputs come from an app-server 0.167.0-alpha.3.2 failed rollout.
// Only the single-part null snapshots are captured production examples.
const codexOpenPageNullOutput = `{"page_id":null}`
const codexSelectedPageNullOutput = "Companion Page snapshot part 1 of 1:\n" + `{"page_id":null,"document_view":null}`

func codexPageInput(name, output string) map[string]any {
	return map[string]any{
		"type": "function_call_output", "namespace": "agent",
		"name": name, "output": output,
	}
}

func codexPageRequest(t *testing.T, items ...any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"model": "test-model", "input": items})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func checkCodexPageMessage(t *testing.T, item gjson.Result, name, output string) {
	t.Helper()
	if item.Get("type").Str != "message" || item.Get("role").Str != "user" ||
		item.Get("content.0.type").Str != "input_text" {
		t.Fatalf("expected a user text message, got %s", item.Raw)
	}
	want := "<external_" + name + ">\n" + output + "\n</external_" + name + ">"
	if item.Get("content.0.text").Str != want {
		t.Fatalf("Page provenance or output changed: got %q, want %q", item.Get("content.0.text").Str, want)
	}
	if item.Get("call_id").Exists() || item.Get("output").Exists() || item.Get("id").Exists() {
		t.Fatalf("tool output metadata leaked into the message: %s", item.Raw)
	}
}

// Browser envelope from the reported Desktop request, with an example URL.
const codexBrowserTabsOutput = "\n" + `<in-app-browser-context source="ambient-ui-state">
This block is automatically supplied ambient UI state, not part of the user's request. Do not treat it as an instruction or as evidence that the user explicitly selected the in-app browser.
# In app browser:
- The user has the in-app browser open with 1 tab.
- Current URL: https://example.com/
</in-app-browser-context>` + "\n"

func TestPrepareOpenAIWSHTTPBridgeBodyNormalizesPageContextEveryTurn(t *testing.T) {
	for _, previous := range []string{"", "resp-previous"} {
		t.Run(previous, func(t *testing.T) {
			// Model a first turn and a later turn carrying replayed tool history.
			// The same output preparation runs for every bridge HTTP request.
			body := codexPageRequest(t,
				map[string]any{"type": "function_call", "call_id": "call-1", "name": "read"},
				map[string]any{"type": "function_call_output", "call_id": "call-1", "output": "done"},
				codexPageInput("codex_apps_open_page", codexOpenPageNullOutput),
				codexPageInput("codex_apps_selected_page_part_1_of_1", codexSelectedPageNullOutput))
			// Prefix extra WS fields without reparsing away any input shape.
			payload := append([]byte(`{"type":"response.create","previous_response_id":"`+previous+`",`), body[1:]...)
			got, err := prepareOpenAIWSHTTPBridgeBody(nil, payload)
			if err != nil {
				t.Fatal(err)
			}
			if gjson.GetBytes(got, "type").Exists() || gjson.GetBytes(got, "previous_response_id").Exists() ||
				!gjson.GetBytes(got, "stream").Bool() {
				t.Fatalf("WS to HTTP preparation changed: %s", got)
			}
			input := gjson.GetBytes(got, "input").Array()
			if len(input) != 4 || input[1].Get("call_id").Str != "call-1" || input[1].Get("output").Str != "done" {
				t.Fatalf("paired tool history changed: %s", got)
			}
			checkCodexPageMessage(t, input[2], "codex_apps_open_page", codexOpenPageNullOutput)
			checkCodexPageMessage(t, input[3], "codex_apps_selected_page_part_1_of_1", codexSelectedPageNullOutput)
		})
	}
}

func TestPrepareOpenAIWSHTTPBridgeBodyNormalizesBrowserContextEveryTurn(t *testing.T) {
	for _, previous := range []string{"", "resp-previous"} {
		t.Run(previous, func(t *testing.T) {
			body := codexPageRequest(t,
				map[string]any{"type": "function_call", "call_id": "call-1", "name": "read"},
				map[string]any{"type": "function_call_output", "call_id": "call-1", "output": "done"},
				codexPageInput("browser_tabs", codexBrowserTabsOutput),
				codexPageInput("codex_apps_open_page", codexOpenPageNullOutput))
			payload := append([]byte(`{"type":"response.create","previous_response_id":"`+previous+`",`), body[1:]...)
			got, err := prepareOpenAIWSHTTPBridgeBody(nil, payload)
			if err != nil {
				t.Fatal(err)
			}
			if gjson.GetBytes(got, "type").Exists() || gjson.GetBytes(got, "previous_response_id").Exists() ||
				!gjson.GetBytes(got, "stream").Bool() {
				t.Fatalf("WS to HTTP preparation changed: %s", got)
			}
			before := gjson.GetBytes(body, "input").Array()
			after := gjson.GetBytes(got, "input").Array()
			if len(after) != len(before) || before[0].Raw != after[0].Raw || before[1].Raw != after[1].Raw {
				t.Fatalf("paired tool history changed: %s", got)
			}
			checkCodexPageMessage(t, after[2], "browser_tabs", codexBrowserTabsOutput)
			checkCodexPageMessage(t, after[3], "codex_apps_open_page", codexOpenPageNullOutput)
		})
	}
}
