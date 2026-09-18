package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

// Register only on the authenticated, rate-limited admin group.
func registerPhotonthinxUsageReportRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	reports := admin.Group("/internal/reports/usage")
	reports.GET("/monthly", h.Admin.Usage.PhotonthinxMonthly)
	reports.GET("/trend", h.Admin.Usage.PhotonthinxTrend)
}
