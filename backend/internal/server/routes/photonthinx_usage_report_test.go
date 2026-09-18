package routes

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

type photonthinxUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r *photonthinxUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	return r.user, nil
}
func (r *photonthinxUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}
func TestPhotonthinxRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "report-test-secret", ExpireHour: 1}}
	auth := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	user := &service.User{ID: 1, Email: "user@test.com", Role: service.RoleUser, Status: service.StatusActive}
	users := service.NewUserService(&photonthinxUserRepo{user: user}, nil, nil, nil)
	token, err := auth.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	r := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{Usage: admin.NewUsageHandler(service.NewUsageService(nil, nil, nil, nil), nil, nil, nil)}}
	RegisterAdminRoutes(r.Group("/api/v1"), h, middleware.NewAdminAuthMiddleware(auth, users, nil, nil), middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, middleware.NewPanelRateLimiter(nil, nil))
	for _, endpoint := range []string{"monthly", "trend"} {
		path := "/api/v1/admin/internal/reports/usage/" + endpoint
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 401, w.Code)
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, 403, w.Code)
	}
}
