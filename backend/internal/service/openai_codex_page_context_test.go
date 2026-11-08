package service

import (
	"encoding/json"
	"strings"
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
	want := "<external_" + name + ">" + output + "</external_" + name + ">"
	if item.Get("content.0.text").Str != want {
		t.Fatalf("Page provenance or output changed: got %q, want %q", item.Get("content.0.text").Str, want)
	}
	if item.Get("call_id").Exists() || item.Get("output").Exists() || item.Get("id").Exists() {
		t.Fatalf("tool output metadata leaked into the message: %s", item.Raw)
	}
}

func TestNormalizeCodexPageContextRealNullSnapshots(t *testing.T) {
	open := codexPageInput("codex_apps_open_page", codexOpenPageNullOutput)
	open["id"] = "client-item"
	body := codexPageRequest(t, open,
		map[string]any{"type": "message", "role": "user", "content": "between"},
		codexPageInput("codex_apps_selected_page_part_1_of_1", codexSelectedPageNullOutput))
	got, changed := NormalizeCodexPageContext(body)
	if !changed {
		t.Fatal("real standalone Page snapshots were not normalized")
	}
	input := gjson.GetBytes(got, "input").Array()
	if len(input) != 3 || input[1].Get("content").Str != "between" {
		t.Fatalf("input order changed: %s", got)
	}
	checkCodexPageMessage(t, input[0], "codex_apps_open_page", codexOpenPageNullOutput)
	checkCodexPageMessage(t, input[2], "codex_apps_selected_page_part_1_of_1", codexSelectedPageNullOutput)
	again, changed := NormalizeCodexPageContext(got)
	if changed || string(again) != string(got) {
		t.Fatal("normalization must be idempotent")
	}
}

func TestNormalizeCodexPageContextPreservesOtherInputAndFields(t *testing.T) {
	// The paired Page output, delegation, unknown standalone result and large
	// integer must survive independently of converting the supported Page item.
	const body = `{"model":"test-model","previous_response_id":"resp-1","metadata":{"integer":9007199254740993},"input":[` +
		`{"type":"function_call_output","namespace":"agent","name":"codex_apps_open_page","output":"{\"page_id\":null}"},` +
		`{"type":"function_call","call_id":"call-1","name":"read"},` +
		`{"type":"function_call_output","call_id":"call-1","output":"done"},` +
		`{"type":"function_call_output","namespace":"agent","name":"codex_apps_open_page","call_id":"call-2","output":"{\"page_id\":null}"},` +
		`{"type":"function_call_output","namespace":"codex_app","name":"create_thread","output":"<codex_delegation><source_thread_id>thread-1</source_thread_id><input>work</input></codex_delegation>"},` +
		`{"type":"function_call_output","name":"unknown","output":"standalone"},` +
		`{"type":"item_reference","id":"item-1"}]}`
	got, changed := NormalizeCodexPageContext([]byte(body))
	if !changed || gjson.GetBytes(got, "previous_response_id").Str != "resp-1" ||
		gjson.GetBytes(got, "metadata.integer").Raw != "9007199254740993" {
		t.Fatalf("request fields changed: %s", got)
	}
	before := gjson.Get(body, "input").Array()
	after := gjson.GetBytes(got, "input").Array()
	if len(before) != len(after) {
		t.Fatal("input items were removed")
	}
	for i := 1; i < len(before); i++ {
		if before[i].Raw != after[i].Raw {
			t.Fatalf("unrelated input %d changed: %s", i, after[i].Raw)
		}
	}
}

func TestNormalizeCodexPageContextRejectsOtherShapes(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string]any)
	}{
		{"paired", func(item map[string]any) { item["call_id"] = "call-1" }},
		{"null call id", func(item map[string]any) { item["call_id"] = nil }},
		{"empty call id", func(item map[string]any) { item["call_id"] = "" }},
		{"whitespace call id", func(item map[string]any) { item["call_id"] = " " }},
		{"numeric call id", func(item map[string]any) { item["call_id"] = 1 }},
		{"other namespace", func(item map[string]any) { item["namespace"] = "codex_app" }},
		{"other type", func(item map[string]any) { item["type"] = "custom_tool_call_output" }},
		{"other name", func(item map[string]any) { item["name"] = "codex_apps_other" }},
		{"padded name", func(item map[string]any) { item["name"] = " codex_apps_open_page" }},
		{"nonstring output", func(item map[string]any) { item["output"] = map[string]any{"page_id": nil} }},
		{"invalid snapshot", func(item map[string]any) { item["output"] = `{"page_id":` }},
		{"wrong snapshot", func(item map[string]any) { item["output"] = `{"other":null}` }},
		{"numeric page id", func(item map[string]any) { item["output"] = `{"page_id":1}` }},
		{"duplicate page id", func(item map[string]any) { item["output"] = `{"page_id":null,"page_id":1}` }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := codexPageInput("codex_apps_open_page", codexOpenPageNullOutput)
			tc.edit(item)
			body := codexPageRequest(t, item)
			got, changed := NormalizeCodexPageContext(body)
			if changed || string(got) != string(body) {
				t.Fatalf("unsupported input was rewritten: %s", got)
			}
		})
	}
}

func TestNormalizeCodexPageContextSyntheticSelectedSnapshots(t *testing.T) {
	// Synthetic non-null and split snapshots test preservation and boundaries;
	// they are not evidence of Desktop's current non-null/multipart wire form.
	full := `{"page_id":"page-1","document_view":{"text":"A & B <tag> 中文","offset":9007199254740993}}`
	cases := []struct {
		name, output string
		changed      bool
	}{
		{"codex_apps_selected_page_part_1_of_1", "Companion Page snapshot part 1 of 1:\n" + full, true},
		{"codex_apps_selected_page_part_1_of_2", "Companion Page snapshot part 1 of 2:\n" + `{"page_id":"page-1",`, true},
		{"codex_apps_selected_page_part_2_of_2", "Companion Page snapshot part 2 of 2:\n" + `"document_view":null}`, true},
		{"codex_apps_selected_page_part_0_of_1", "Companion Page snapshot part 0 of 1:\n" + full, false},
		{"codex_apps_selected_page_part_2_of_1", "Companion Page snapshot part 2 of 1:\n" + full, false},
		{"codex_apps_selected_page_part_01_of_1", "Companion Page snapshot part 01 of 1:\n" + full, false},
		{"codex_apps_selected_page_part_1_of_1_extra", codexSelectedPageNullOutput, false},
		{"codex_apps_selected_page_part_1_of_99999999999999999999", codexSelectedPageNullOutput, false},
		{"codex_apps_selected_page_part_2_of_2", "Companion Page snapshot part 1 of 2:\n" + full, false},
		{"codex_apps_selected_page_part_1_of_2", "Companion Page snapshot part 1 of 2:\n ", false},
		{"codex_apps_selected_page_part_1_of_1", "Companion Page snapshot part 1 of 1:\n" + `{"page_id":null}`, false},
		{"codex_apps_selected_page_part_1_of_1", "Companion Page snapshot part 1 of 1:\n" + `{"page_id":null,"document_view":`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.output, func(t *testing.T) {
			body := codexPageRequest(t, codexPageInput(tc.name, tc.output))
			got, changed := NormalizeCodexPageContext(body)
			if changed != tc.changed {
				t.Fatalf("changed=%v, want %v", changed, tc.changed)
			}
			if changed {
				checkCodexPageMessage(t, gjson.GetBytes(got, "input.0"), tc.name, tc.output)
			} else if string(got) != string(body) {
				t.Fatal("unrecognized snapshot changed")
			}
		})
	}
}

func TestNormalizeCodexPageContextJSONBoundaries(t *testing.T) {
	body := string(codexPageRequest(t, codexPageInput("codex_apps_open_page", codexOpenPageNullOutput)))
	for name, value := range map[string]string{
		"escaped type":      strings.Replace(body, "function_call_output", `\u0066unction_call_output`, 1),
		"escaped name":      strings.Replace(body, "codex_apps_open_page", `\u0063odex_apps_open_page`, 1),
		"escaped namespace": strings.Replace(body, "agent", `\u0061gent`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, changed := NormalizeCodexPageContext([]byte(value)); !changed {
				t.Fatal("escaped discriminator was not recognized")
			}
		})
	}
	for name, value := range map[string]string{
		"duplicate input":         strings.Replace(body, `"input":`, `"input":[],"input":`, 1),
		"duplicate discriminator": strings.Replace(body, `"namespace":"agent"`, `"namespace":"agent","namespace":"other"`, 1),
		"duplicate call id":       strings.Replace(body, `"namespace":"agent"`, `"call_id":"call-1","call_id":"","namespace":"agent"`, 1),
		"duplicate output":        strings.Replace(body, `"output":`, `"output":"unrelated","output":`, 1),
		"trailing JSON":           body + `{}`,
		"truncated JSON":          strings.TrimSuffix(body, "}"),
	} {
		t.Run(name, func(t *testing.T) {
			got, changed := NormalizeCodexPageContext([]byte(value))
			if changed || string(got) != value {
				t.Fatal("ambiguous or invalid JSON was rewritten")
			}
		})
	}
}

func TestNormalizeCodexPageContextNoncandidateFastPath(t *testing.T) {
	body := []byte(`{"input":[{"type":"function_call_output","call_id":"call-1","output":"done"}]}`)
	allocs := testing.AllocsPerRun(10, func() {
		got, changed := NormalizeCodexPageContext(body)
		if changed || &got[0] != &body[0] {
			t.Fatal("noncandidate input must be returned untouched")
		}
	})
	if allocs != 0 {
		t.Fatalf("noncandidate fast path allocated %v times", allocs)
	}
}
