//go:build integration

package repository

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	report "github.com/Wei-Shaw/sub2api/internal/photonthinx/usagereport"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPhotonthinxMonthlyAggregation(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	_, err := tx.ExecContext(ctx, "DELETE FROM usage_logs")
	require.NoError(t, err)
	repo := newUsageLogRepositoryWithSQL(client, tx)
	acc := mustCreateAccount(t, client, &service.Account{Name: "report"})
	first := mustCreateUser(t, client, &service.User{Email: "first@report.test"})
	second := mustCreateUser(t, client, &service.User{Email: "second@report.test"})
	third := mustCreateUser(t, client, &service.User{Email: "zero@report.test"})
	write := func(id int64, date string, cost float64, tokens int) {
		key := mustCreateApiKey(t, client, &service.APIKey{UserID: id, Key: fmt.Sprintf("report-%d-%s-%d", id, date, tokens), Name: "report"})
		at, e := time.Parse(time.RFC3339, date)
		require.NoError(t, e)
		multiplier := 2.0
		statsCost := 0.25
		_, e = repo.Create(ctx, &service.UsageLog{UserID: id, APIKeyID: key.ID, AccountID: acc.ID, Model: "report", ActualCost: cost, TotalCost: 0.5, AccountStatsCost: &statsCost, AccountRateMultiplier: &multiplier, InputTokens: tokens, OutputTokens: 2, CacheCreationTokens: 3, CacheReadTokens: 4, CreatedAt: at})
		require.NoError(t, e)
	}
	write(first.ID, "2024-02-01T00:00:00+08:00", 0.1, 10)
	write(first.ID, "2024-02-29T23:59:59+08:00", 0.2, 10)
	write(second.ID, "2024-02-15T00:00:00+08:00", 0.3, 1)
	write(third.ID, "2024-02-15T00:00:00+08:00", 0, 0)
	write(second.ID, "2024-01-31T23:59:59+08:00", 8, 1)
	write(second.ID, "2024-03-01T00:00:00+08:00", 9, 1)
	// Existing records of deleted/disabled users still count.
	_, err = tx.ExecContext(ctx, "UPDATE users SET deleted_at=NOW(), status='disabled' WHERE id=$1", second.ID)
	require.NoError(t, err)
	now := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	q, err := report.Parse(url.Values{"month": {"2024-02"}, "page_size": {"1"}}, now, false)
	require.NoError(t, err)
	page, err := repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.Equal(t, int64(3), page.Total)
	require.Equal(t, int64(3), page.Summary.ActiveUsers)
	require.Equal(t, int64(4), page.Summary.Requests)
	require.InDelta(t, 0.6, page.Summary.ActualCost, 1e-12)
	require.InDelta(t, 2, page.Summary.AccountCost, 1e-12)
	require.Equal(t, int64(57), page.Summary.TotalTokens)
	require.Equal(t, first.ID, page.Items[0].UserID)
	require.InDelta(t, 0.5, page.Items[0].CostShare, 1e-12)
	sum := 0.0
	for i := 1; i <= 3; i++ {
		q.Page = i
		p, e := repo.PhotonthinxMonthly(ctx, q)
		require.NoError(t, e)
		sum += p.Items[0].ActualCost
		require.Equal(t, page.Summary, p.Summary)
	}
	require.InDelta(t, page.Summary.ActualCost, sum, 1e-12)
	q.Page = 99
	p, err := repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.Empty(t, p.Items)
	require.Equal(t, page.Summary, p.Summary)
	require.Equal(t, int64(3), p.Total)
	q.Page = 1
	q.Search = "second"
	p, err = repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.Equal(t, int64(1), p.Total)
	require.Equal(t, second.ID, p.Items[0].UserID)
	require.Equal(t, page.Summary, p.Summary)
	q.Search = "%"
	p, err = repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.Zero(t, p.Total)
	q.Search = ""
	q.UserID = first.ID
	q.Granularity = "day"
	trend, err := repo.PhotonthinxTrend(ctx, q)
	require.NoError(t, err)
	sum = 0
	for _, point := range trend.Points {
		sum += point.ActualCost
	}
	require.InDelta(t, 0.3, sum, 1e-12)
	require.Len(t, trend.Points, 2)
	require.Equal(t, "2024-02-29", trend.Points[1].Date)
	q.UserID = 0
	q.Granularity = "month"
	trend, err = repo.PhotonthinxTrend(ctx, q)
	require.NoError(t, err)
	require.Len(t, trend.Points, 1)
	require.InDelta(t, 0.6, trend.Points[0].ActualCost, 1e-12)
	// Stable ID tie-break and unrestricted total, including the page after Top 200.
	for i := 0; i < 205; i++ {
		u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("extra%d@report.test", i)})
		write(u.ID, "2024-02-10T00:00:00+08:00", 0, 0)
	}
	q.Page = 3
	q.PageSize = 100
	p, err = repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.Equal(t, int64(208), p.Total)
	require.Len(t, p.Items, 8)
	for i := 1; i < len(p.Items); i++ {
		require.Less(t, p.Items[i-1].UserID, p.Items[i].UserID)
	}
	q.Sort = "requests"
	q.Order = "asc"
	q.Page = 1
	p, err = repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.Equal(t, int64(1), p.Items[0].Requests)
	q.Sort = "total_tokens"
	q.Order = "desc"
	p, err = repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.Equal(t, first.ID, p.Items[0].UserID)
	// Historical rows without cost snapshots use the existing fallback formula.
	_, err = tx.ExecContext(ctx, "UPDATE usage_logs SET account_stats_cost=NULL, account_rate_multiplier=NULL WHERE user_id=$1", first.ID)
	require.NoError(t, err)
	p, err = repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.InDelta(t, 1.0, p.Items[0].AccountCost, 1e-12)
}

func TestPhotonthinxEmptyRetainedHistory(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	_, err := tx.ExecContext(ctx, "DELETE FROM usage_logs")
	require.NoError(t, err)
	repo := newUsageLogRepositoryWithSQL(tx.Client(), tx)
	q, err := report.Parse(url.Values{"month": {"2024-02"}}, time.Now(), false)
	require.NoError(t, err)
	monthly, err := repo.PhotonthinxMonthly(ctx, q)
	require.NoError(t, err)
	require.Empty(t, monthly.Items)
	require.Zero(t, monthly.Total)
	require.Nil(t, monthly.AvailableFrom)
	require.Equal(t, report.Summary{}, monthly.Summary)
	require.True(t, monthly.HistoryIncomplete)
	trend, err := repo.PhotonthinxTrend(ctx, q)
	require.NoError(t, err)
	require.Empty(t, trend.Points)
	require.Nil(t, trend.AvailableFrom)
}
