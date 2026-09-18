package usagereport

import (
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
	"time"
)

func TestParseShanghaiMonths(t *testing.T) {
	now := time.Date(2026, 9, 30, 16, 1, 0, 0, time.UTC)
	q, err := Parse(url.Values{}, now, false)
	require.NoError(t, err)
	require.Equal(t, "2026-10", q.Month)
	require.Equal(t, "2026-09-30T16:00:00Z", q.Start.UTC().Format(time.RFC3339))
	require.Equal(t, "actual_cost", q.Sort)
	for _, tc := range []struct {
		month, end string
		days       int
	}{{"2024-02", "2024-03-01", 29}, {"2025-02", "2025-03-01", 28}, {"2025-12", "2026-01-01", 31}} {
		q, err = Parse(url.Values{"month": {tc.month}}, now, false)
		require.NoError(t, err)
		require.Equal(t, tc.end, q.End.Format("2006-01-02"))
		require.Equal(t, tc.days, int(q.End.Sub(q.Start).Hours()/24))
	}
	q, err = Parse(url.Values{"month": {"2026-01"}, "granularity": {"month"}, "months": {"12"}}, now, true)
	require.NoError(t, err)
	require.Equal(t, "2025-02", q.Start.Format("2006-01"))
	for _, v := range []url.Values{{"month": {"2026-13"}}, {"month": {"2026-11"}}, {"page": {"0"}}, {"page_size": {"101"}}, {"sort_by": {"password"}}, {"sort_order": {"bad"}}, {"user_id": {"-1"}}, {"months": {"13"}}, {"granularity": {"hour"}}, {"granularity": {"day"}, "months": {"2"}}} {
		_, err = Parse(v, now, true)
		require.Error(t, err, "%v", v)
	}
}

func TestFillTrendDistinguishesMissingHistoryAndFuture(t *testing.T) {
	now := time.Date(2026, 9, 18, 9, 0, 0, 0, Shanghai)
	q, err := Parse(url.Values{"month": {"2026-09"}}, now, true)
	require.NoError(t, err)
	first := time.Date(2026, 9, 3, 13, 15, 0, 0, Shanghai)
	result := Trend{Metadata: Metadata{AvailableFrom: &first}, Points: []Point{{Date: "2026-09-03", Metrics: Metrics{ActualCost: 0.3, Requests: 2}}}}
	FillTrend(&result, q)
	require.Len(t, result.Points, 18)
	require.False(t, result.Points[0].Available)
	require.True(t, result.Points[2].Available)
	require.True(t, result.Points[2].Partial)
	require.Equal(t, 0.3, result.Points[2].ActualCost)
	require.True(t, result.Points[3].Available)
	require.Zero(t, result.Points[3].Requests)
	require.True(t, result.HistoryIncomplete)
	require.True(t, result.Current)
	q, err = Parse(url.Values{"month": {"2026-09"}, "granularity": {"month"}}, now, true)
	require.NoError(t, err)
	result = Trend{Metadata: Metadata{AvailableFrom: &first}}
	FillTrend(&result, q)
	require.Len(t, result.Points, 12)
	require.False(t, result.Points[10].Available)
	require.True(t, result.Points[11].Partial)
	result = Trend{}
	FillTrend(&result, q)
	require.False(t, result.Points[11].Available)
}
