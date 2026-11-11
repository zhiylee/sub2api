//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Existing update/rollback tests do not use the independent upstream channel.
func (*systemHandlerUpdateServiceStub) CheckUpstreamRelease(context.Context, bool) *service.UpstreamReleaseInfo {
	return &service.UpstreamReleaseInfo{}
}

type upstreamHandlerServiceStub struct {
	systemHandlerUpdateServiceStub
	forces []bool
}

func (s *upstreamHandlerServiceStub) CheckUpstreamRelease(_ context.Context, force bool) *service.UpstreamReleaseInfo {
	s.forces = append(s.forces, force)
	return &service.UpstreamReleaseInfo{BaseVersion: "0.2.21", LatestVersion: "0.2.22", HasNewRelease: true}
}

func TestSystemHandlerUpstreamReleaseIsReadOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &upstreamHandlerServiceStub{}
	handler := NewSystemHandler(svc, nil)
	router := gin.New()
	router.GET("/upstream-release", handler.GetUpstreamRelease)
	for _, query := range []string{"", "?force=true"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/upstream-release"+query, nil))
		require.Equal(t, http.StatusOK, rec.Code)
		var envelope struct {
			Data service.UpstreamReleaseInfo `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		require.Equal(t, "0.2.21", envelope.Data.BaseVersion)
		require.True(t, envelope.Data.HasNewRelease)
		require.NotContains(t, rec.Body.String(), "assets")
	}
	require.Equal(t, []bool{false, true}, svc.forces)
	require.Empty(t, svc.checkForces)
	require.Zero(t, svc.performCall)
	require.Zero(t, svc.rollbackCall)
	require.Zero(t, svc.rollbackToCall)
}
