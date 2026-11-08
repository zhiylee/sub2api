package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func TestCodexPageContextNormalizesBeforeBootstrapAndHTTPValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, bootstrap := range []string{"", "delegation", "automation"} {
		t.Run(bootstrap, func(t *testing.T) {
			input := []any{
				map[string]any{"type": "function_call_output", "namespace": "agent", "name": "codex_apps_open_page", "output": `{"page_id":null}`},
				map[string]any{"type": "function_call_output", "namespace": "agent", "name": "codex_apps_selected_page_part_1_of_1", "output": "Companion Page snapshot part 1 of 1:\n" + `{"page_id":null,"document_view":null}`},
			}
			switch bootstrap {
			case "delegation":
				input = append(input, map[string]any{"type": "function_call_output", "namespace": "codex_app", "name": "create_thread", "output": delegationEnvelope})
			case "automation":
				input = append(input, map[string]any{"type": "function_call_output", "namespace": "codex_app", "name": "automation_update", "output": `<heartbeat><automation_id>wiki</automation_id></heartbeat>`})
			}
			body, err := json.Marshal(map[string]any{"model": "test-model", "input": input})
			require.NoError(t, err)
			body, changed := service.NormalizeCodexPageContext(body)
			require.True(t, changed)
			body, _ = normalizeCodexAutomationBootstrap(body)
			body, _ = normalizeCodexDelegationBootstrap(body)
			for _, item := range gjson.GetBytes(body, "input").Array() {
				require.Equal(t, "message", item.Get("type").Str)
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			require.True(t, (&OpenAIGatewayHandler{}).validateFunctionCallOutputRequest(c, body, zap.NewNop()))
			require.False(t, recorder.Flushed)
			require.Empty(t, recorder.Body.String())
		})
	}
}
