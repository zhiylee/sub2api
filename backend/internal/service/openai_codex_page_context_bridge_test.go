package service

import (
	"testing"

	"github.com/tidwall/gjson"
)

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
