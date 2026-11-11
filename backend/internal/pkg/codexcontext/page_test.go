package codexcontext

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

func TestNormalizePageContextBrowserTabs(t *testing.T) {
	for name, output := range map[string]string{
		"reported envelope": codexBrowserTabsOutput,
		"no outer space":    strings.TrimSpace(codexBrowserTabsOutput),
		"crlf and unicode": strings.ReplaceAll(
			strings.ReplaceAll(codexBrowserTabsOutput, "# In app browser:", "# In app browser: 中文 & <tag>"), "\n", "\r\n"),
	} {
		t.Run(name, func(t *testing.T) {
			body := codexPageRequest(t,
				map[string]any{"type": "message", "role": "user", "content": "before"},
				codexPageInput("browser_tabs", output),
				codexPageInput("codex_apps_open_page", codexOpenPageNullOutput),
				codexPageInput("codex_apps_selected_page_part_1_of_1", codexSelectedPageNullOutput),
				map[string]any{"type": "message", "role": "user", "content": "只回复 OK"})
			got, changed := NormalizePageContext(body)
			if !changed {
				t.Fatal("standalone browser context was not normalized")
			}
			input := gjson.GetBytes(got, "input").Array()
			if len(input) != 5 || input[0].Get("content").Str != "before" || input[4].Get("content").Str != "只回复 OK" {
				t.Fatalf("input order changed: %s", got)
			}
			checkCodexPageMessage(t, input[1], "browser_tabs", output)
			checkCodexPageMessage(t, input[2], "codex_apps_open_page", codexOpenPageNullOutput)
			checkCodexPageMessage(t, input[3], "codex_apps_selected_page_part_1_of_1", codexSelectedPageNullOutput)
			again, changed := NormalizePageContext(got)
			if changed || string(again) != string(got) {
				t.Fatal("normalization must be idempotent")
			}
		})
	}
	// No Pages discriminator is present in the minimal reproducer.
	body := codexPageRequest(t, codexPageInput("browser_tabs", codexBrowserTabsOutput))
	got, changed := NormalizePageContext(body)
	if !changed {
		t.Fatal("browser-only context was skipped by the fast path")
	}
	checkCodexPageMessage(t, gjson.GetBytes(got, "input.0"), "browser_tabs", codexBrowserTabsOutput)
}

func TestNormalizePageContextRejectsOtherBrowserShapes(t *testing.T) {
	for name, edit := range map[string]func(map[string]any){
		"paired":             func(item map[string]any) { item["call_id"] = "call-1" },
		"null call id":       func(item map[string]any) { item["call_id"] = nil },
		"empty call id":      func(item map[string]any) { item["call_id"] = "" },
		"whitespace call id": func(item map[string]any) { item["call_id"] = " " },
		"numeric call id":    func(item map[string]any) { item["call_id"] = 1 },
		"other namespace":    func(item map[string]any) { item["namespace"] = "browser" },
		"other name":         func(item map[string]any) { item["name"] = "browser_tabs_extra" },
		"other type":         func(item map[string]any) { item["type"] = "custom_tool_call_output" },
		"nonstring output":   func(item map[string]any) { item["output"] = []any{codexBrowserTabsOutput} },
		"ordinary result":    func(item map[string]any) { item["output"] = "Current URL: https://example.com/" },
		"other source": func(item map[string]any) {
			item["output"] = strings.Replace(codexBrowserTabsOutput, "ambient-ui-state", "tool-result", 1)
		},
		"missing source": func(item map[string]any) {
			item["output"] = strings.Replace(codexBrowserTabsOutput, ` source="ambient-ui-state"`, "", 1)
		},
		"missing closing tag": func(item map[string]any) {
			item["output"] = strings.Replace(codexBrowserTabsOutput, "</in-app-browser-context>", "", 1)
		},
		"empty envelope": func(item map[string]any) {
			item["output"] = "<in-app-browser-context source=\"ambient-ui-state\">\n \t</in-app-browser-context>"
		},
		"text before envelope": func(item map[string]any) { item["output"] = "result:" + codexBrowserTabsOutput },
		"text after envelope":  func(item map[string]any) { item["output"] = codexBrowserTabsOutput + "result" },
	} {
		t.Run(name, func(t *testing.T) {
			item := codexPageInput("browser_tabs", codexBrowserTabsOutput)
			edit(item)
			body := codexPageRequest(t, item)
			got, changed := NormalizePageContext(body)
			if changed || string(got) != string(body) {
				t.Fatalf("unsupported browser input was rewritten: %s", got)
			}
		})
	}
}

func TestNormalizePageContextBrowserHistoryAndJSONBoundaries(t *testing.T) {
	paired := codexPageInput("browser_tabs", codexBrowserTabsOutput)
	paired["call_id"] = "call-2"
	body := codexPageRequest(t,
		codexPageInput("browser_tabs", codexBrowserTabsOutput),
		map[string]any{"type": "function_call", "call_id": "call-1", "name": "read"},
		map[string]any{"type": "function_call_output", "call_id": "call-1", "output": "done"},
		paired,
		map[string]any{"type": "function_call_output", "name": "unknown", "output": "standalone"})
	body = append([]byte(`{"previous_response_id":"resp-1","metadata":{"integer":9007199254740993},`), body[1:]...)
	for name, value := range map[string]string{
		"history":           string(body),
		"escaped type":      strings.ReplaceAll(string(body), "function_call_output", `\u0066unction_call_output`),
		"escaped name":      strings.ReplaceAll(string(body), "browser_tabs", `\u0062rowser_tabs`),
		"escaped namespace": strings.ReplaceAll(string(body), "agent", `\u0061gent`),
	} {
		t.Run(name, func(t *testing.T) {
			got, changed := NormalizePageContext([]byte(value))
			if !changed || gjson.GetBytes(got, "previous_response_id").Str != "resp-1" ||
				gjson.GetBytes(got, "metadata.integer").Raw != "9007199254740993" {
				t.Fatalf("browser context was skipped or request fields changed: %s", got)
			}
			before := gjson.Get(value, "input").Array()
			after := gjson.GetBytes(got, "input").Array()
			if len(before) != len(after) {
				t.Fatal("input items were removed")
			}
			checkCodexPageMessage(t, after[0], "browser_tabs", codexBrowserTabsOutput)
			for i := 1; i < len(before); i++ {
				if before[i].Raw != after[i].Raw {
					t.Fatalf("unrelated input %d changed: %s", i, after[i].Raw)
				}
			}
		})
	}

	body = codexPageRequest(t, codexPageInput("browser_tabs", codexBrowserTabsOutput))
	for name, value := range map[string]string{
		"duplicate input":     strings.Replace(string(body), `"input":`, `"input":[],"input":`, 1),
		"duplicate namespace": strings.Replace(string(body), `"namespace":"agent"`, `"namespace":"agent","namespace":"other"`, 1),
		"duplicate output":    strings.Replace(string(body), `"output":`, `"output":"unrelated","output":`, 1),
		"trailing JSON":       string(body) + `{}`,
		"truncated JSON":      strings.TrimSuffix(string(body), "}"),
	} {
		t.Run(name, func(t *testing.T) {
			got, changed := NormalizePageContext([]byte(value))
			if changed || string(got) != value {
				t.Fatal("ambiguous or invalid browser JSON was rewritten")
			}
		})
	}
}

func TestNormalizePageContextPreservesUpstreamPageShapes(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		emptyCallID  bool
	}{
		{"codex_apps_open_page", codexOpenPageNullOutput, true},
		{"codex_apps_selected_page_part_1_of_1", "Companion Page snapshot part 1 of 1:\r\n" + `{"page_id":null}`, false},
	} {
		item := codexPageInput(tc.name, tc.output)
		if tc.emptyCallID {
			item["call_id"] = ""
		}
		body := codexPageRequest(t, item)
		got, changed := NormalizePageContext(body)
		if !changed {
			t.Fatalf("upstream-supported Page shape was rejected: %s", body)
		}
		checkCodexPageMessage(t, gjson.GetBytes(got, "input.0"), tc.name, tc.output)
	}
}

func TestNormalizePageContextNoncandidateFastPath(t *testing.T) {
	body := []byte(`{"input":[{"type":"function_call_output","call_id":"call-1","output":"done"}]}`)
	allocs := testing.AllocsPerRun(10, func() {
		got, changed := NormalizePageContext(body)
		if changed || &got[0] != &body[0] {
			t.Fatal("noncandidate input must be returned untouched")
		}
	})
	if allocs != 0 {
		t.Fatalf("noncandidate fast path allocated %v times", allocs)
	}
}

func TestNormalizePageContextRejectsNestedDuplicateMembers(t *testing.T) {
	body := codexPageRequest(t, codexPageInput("codex_apps_open_page", codexOpenPageNullOutput))
	body = append([]byte(`{"metadata":{"key":1,"key":2},`), body[1:]...)
	got, changed := NormalizePageContext(body)
	if changed || string(got) != string(body) {
		t.Fatal("ambiguous JSON must be preserved for the normal request validation")
	}
}
