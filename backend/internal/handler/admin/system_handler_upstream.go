package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetUpstreamRelease returns official release metadata for display only.
func (h *SystemHandler) GetUpstreamRelease(c *gin.Context) {
	response.Success(c, h.updateSvc.CheckUpstreamRelease(c.Request.Context(), c.Query("force") == "true"))
}
