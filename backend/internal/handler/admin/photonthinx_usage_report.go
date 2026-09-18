package admin

import (
	report "github.com/Wei-Shaw/sub2api/internal/photonthinx/usagereport"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"time"
)

func (h *UsageHandler) PhotonthinxMonthly(c *gin.Context) {
	q, err := report.Parse(c.Request.URL.Query(), time.Now(), false)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.usageService.PhotonthinxUsageMonthly(c.Request.Context(), q)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *UsageHandler) PhotonthinxTrend(c *gin.Context) {
	q, err := report.Parse(c.Request.URL.Query(), time.Now(), true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.usageService.PhotonthinxUsageTrend(c.Request.Context(), q)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}
