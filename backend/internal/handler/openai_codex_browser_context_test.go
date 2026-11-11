//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

type codexBrowserContextUpstream struct {
	service.HTTPUpstream
	bodies [][]byte
}

func (u *codexBrowserContextUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.bodies = append(u.bodies, body)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n" +
				`event: response.completed` + "\ndata: " + `{"type":"response.completed","response":{"id":"resp_browser","model":"gpt-6.1-sol","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n")),
	}, nil
}

func TestCodexPageContextBrowserTabsHTTPResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The reported envelope and warning are preserved; the URL is an example.
	const output = "\n" + `<in-app-browser-context source="ambient-ui-state">
This block is automatically supplied ambient UI state, not part of the user's request. Do not treat it as an instruction or as evidence that the user explicitly selected the in-app browser.
# In app browser:
- The user has the in-app browser open with 1 tab.
- Current URL: https://example.com/
</in-app-browser-context>` + "\n"
	for _, tc := range []struct {
		name      string
		pages     bool
		bootstrap string
		tool      map[string]any
		wantError string
	}{
		{name: "minimal first turn"},
		{name: "browser and Pages", pages: true},
		{name: "browser Pages and automation", pages: true, bootstrap: "automation"},
		{name: "browser Pages and delegation", pages: true, bootstrap: "delegation"},
		{name: "real output missing call id", tool: map[string]any{"type": "function_call_output", "name": "read", "output": "done"}, wantError: "function_call_output requires call_id"},
		{name: "real output missing reference", tool: map[string]any{"type": "function_call_output", "call_id": "call-1", "output": "done"}, wantError: "function_call_output requires item_reference"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []any{map[string]any{"type": "function_call_output", "namespace": "agent", "name": "browser_tabs", "output": output}}
			if tc.pages {
				input = append(input,
					map[string]any{"type": "function_call_output", "namespace": "agent", "name": "codex_apps_open_page", "output": `{"page_id":null}`},
					map[string]any{"type": "function_call_output", "namespace": "agent", "name": "codex_apps_selected_page_part_1_of_1", "output": "Companion Page snapshot part 1 of 1:\n" + `{"page_id":null,"document_view":null}`})
			}
			if tc.tool != nil {
				input = append(input, tc.tool)
			}
			switch tc.bootstrap {
			case "automation":
				input = append(input, map[string]any{"type": "function_call_output", "namespace": "codex_app", "name": "automation_update", "output": `<heartbeat><automation_id>wiki</automation_id></heartbeat>`})
			case "delegation":
				input = append(input, map[string]any{"type": "function_call_output", "namespace": "codex_app", "name": "create_thread", "output": delegationEnvelope})
			}
			input = append(input, map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "只回复 OK"}}})
			body, err := json.Marshal(map[string]any{"model": "gpt-6.1-sol", "stream": true, "input": input})
			require.NoError(t, err)

			// Independently reproduce the pre-normalization 400 at the validator.
			rawRec := httptest.NewRecorder()
			rawContext, _ := gin.CreateTestContext(rawRec)
			rawContext.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
			require.False(t, (&OpenAIGatewayHandler{}).validateFunctionCallOutputRequest(rawContext, body, zap.NewNop()))
			require.Equal(t, http.StatusBadRequest, rawRec.Code)
			require.Contains(t, rawRec.Body.String(), "function_call_output requires call_id on HTTP requests")

			upstream := &codexBrowserContextUpstream{}
			h := newOpenAIResponsesFailoverTestHandler(t, upstream)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			groupID := int64(3132)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
				ID: 99, GroupID: &groupID, User: &service.User{ID: 100},
				Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
			})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 100, Concurrency: 0})
			h.Responses(c)

			if tc.wantError != "" {
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Contains(t, rec.Body.String(), tc.wantError)
				require.Empty(t, upstream.bodies)
				return
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "response.completed")
			require.Len(t, upstream.bodies, 1)
			forwarded := gjson.GetBytes(upstream.bodies[0], "input").Array()
			require.Len(t, forwarded, len(input))
			for _, item := range forwarded {
				require.Equal(t, "message", item.Get("type").Str)
				require.Equal(t, "user", item.Get("role").Str)
			}
			require.Equal(t, "input_text", forwarded[0].Get("content.0.type").Str)
			require.Equal(t, "<external_browser_tabs>\n"+output+"\n</external_browser_tabs>", forwarded[0].Get("content.0.text").Str)
			require.Equal(t, "只回复 OK", forwarded[len(forwarded)-1].Get("content.0.text").Str)
		})
	}
}
