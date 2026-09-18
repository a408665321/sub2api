package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPhotonthinxReportValidationAndFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewUsageHandler(service.NewUsageService(nil, nil, nil, nil), nil, nil, nil)
	r.GET("/monthly", h.PhotonthinxMonthly)
	r.GET("/trend", h.PhotonthinxTrend)
	for _, path := range []string{"/monthly?month=bad", "/monthly?page_size=999", "/monthly?sort_by=secret", "/trend?user_id=-1", "/trend?months=13", "/trend?granularity=day&months=2"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 400, w.Code, path)
	}
	for _, path := range []string{"/monthly?month=2024-02", "/trend?month=2024-02"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 500, w.Code)
		require.NotContains(t, w.Body.String(), "\"summary\"")
	}
}
