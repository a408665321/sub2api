package service

import (
	"context"
	"errors"
	report "github.com/Wei-Shaw/sub2api/internal/photonthinx/usagereport"
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
	"time"
)

type photonthinxReportRepo struct {
	UsageLogRepository
	query report.Query
	fail  error
}

func (r *photonthinxReportRepo) PhotonthinxMonthly(ctx context.Context, q report.Query) (*report.Monthly, error) {
	r.query = q
	return &report.Monthly{}, r.fail
}
func (r *photonthinxReportRepo) PhotonthinxTrend(ctx context.Context, q report.Query) (*report.Trend, error) {
	r.query = q
	first := q.Start
	return &report.Trend{Metadata: report.Metadata{AvailableFrom: &first}}, r.fail
}
func TestPhotonthinxReportService(t *testing.T) {
	repo := &photonthinxReportRepo{}
	svc := NewUsageService(repo, nil, nil, nil)
	q, e := report.Parse(url.Values{"month": {"2024-02"}}, time.Now(), true)
	require.NoError(t, e)
	result, e := svc.PhotonthinxUsageTrend(context.Background(), q)
	require.NoError(t, e)
	require.Len(t, result.Points, 29)
	require.True(t, result.Points[0].Available)
	_, e = svc.PhotonthinxUsageMonthly(context.Background(), q)
	require.NoError(t, e)
	require.Equal(t, q, repo.query)
	repo.fail = errors.New("database unavailable")
	_, e = svc.PhotonthinxUsageMonthly(context.Background(), q)
	require.ErrorIs(t, e, repo.fail)
	_, e = svc.PhotonthinxUsageTrend(context.Background(), q)
	require.ErrorIs(t, e, repo.fail)
	_, e = NewUsageService(nil, nil, nil, nil).PhotonthinxUsageMonthly(context.Background(), q)
	require.Error(t, e)
}
