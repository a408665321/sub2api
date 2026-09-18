package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	report "github.com/Wei-Shaw/sub2api/internal/photonthinx/usagereport"
)

var _ report.Repository = (*usageLogRepository)(nil)

// All arithmetic/ranking stays NUMERIC in PostgreSQL until the API boundary.
// Do not add usageLogSuccessFilterUL: zero-cost records belong in this report.
const photonthinxUsageMetrics = `
 COALESCE(SUM(ul.actual_cost),0) AS actual_cost,
 COALESCE(SUM(COALESCE(ul.account_stats_cost,ul.total_cost)*COALESCE(ul.account_rate_multiplier,1)),0) AS account_cost,
 COUNT(*) AS requests,
 COALESCE(SUM(ul.input_tokens::bigint),0) AS input_tokens,
 COALESCE(SUM(ul.output_tokens::bigint),0) AS output_tokens,
 COALESCE(SUM(ul.cache_creation_tokens::bigint),0) AS cache_creation_tokens,
 COALESCE(SUM(ul.cache_read_tokens::bigint),0) AS cache_read_tokens,
 COALESCE(SUM(ul.input_tokens::bigint+ul.output_tokens::bigint+ul.cache_creation_tokens::bigint+ul.cache_read_tokens::bigint),0) AS total_tokens`

func (r *usageLogRepository) PhotonthinxMonthly(ctx context.Context, q report.Query) (*report.Monthly, error) {
	// Whitelist even when called outside the HTTP/service layer.
	sortColumn := map[string]string{"actual_cost": "actual_cost", "requests": "requests", "total_tokens": "total_tokens"}[q.Sort]
	if sortColumn == "" {
		return nil, fmt.Errorf("invalid report sort")
	}
	if q.Order != "asc" && q.Order != "desc" {
		return nil, fmt.Errorf("invalid report order")
	}
	order := sortColumn + " " + q.Order + ", total_tokens DESC, user_id ASC"
	query := `WITH grouped AS MATERIALIZED (
 SELECT ul.user_id, ` + photonthinxUsageMetrics + `
 FROM usage_logs ul WHERE ul.created_at >= $1 AND ul.created_at < LEAST($2::timestamptz,$3::timestamptz)
 GROUP BY ul.user_id
 ), summary AS (
 SELECT COUNT(*) AS active_users, COALESCE(SUM(actual_cost),0) AS actual_cost,
 COALESCE(SUM(account_cost),0) AS account_cost, COALESCE(SUM(requests),0) AS requests,
 COALESCE(SUM(input_tokens),0) AS input_tokens, COALESCE(SUM(output_tokens),0) AS output_tokens,
 COALESCE(SUM(cache_creation_tokens),0) AS cache_creation_tokens, COALESCE(SUM(cache_read_tokens),0) AS cache_read_tokens,
 COALESCE(SUM(total_tokens),0) AS total_tokens FROM grouped
 ), filtered AS MATERIALIZED (
 SELECT g.*, COALESCE(u.username,'') AS username, COALESCE(u.email,'') AS email,
 COALESCE(g.actual_cost / NULLIF(s.actual_cost,0),0) AS cost_share
 FROM grouped g LEFT JOIN users u ON u.id=g.user_id CROSS JOIN summary s
 WHERE $4='' OR u.username ILIKE $5 ESCAPE '\' OR u.email ILIKE $5 ESCAPE '\' OR g.user_id::text=$4
 ), paged AS (
 SELECT * FROM filtered ORDER BY ` + order + ` LIMIT $6 OFFSET $7
 ) SELECT json_build_object(
 'summary',(SELECT row_to_json(s) FROM summary s),
 'items',COALESCE((SELECT json_agg(p ORDER BY ` + order + `) FROM paged p),'[]'::json),
 'total',(SELECT COUNT(*) FROM filtered),
 'available_from',(SELECT created_at FROM usage_logs ORDER BY created_at ASC LIMIT 1))`
	literal := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q.Search)
	var raw []byte
	err := scanSingleRow(ctx, r.sql, query, []any{q.Start, q.End, q.Now, q.Search, "%" + literal + "%", q.PageSize, (q.Page - 1) * q.PageSize}, &raw)
	if err != nil {
		return nil, err
	}
	var result report.Monthly
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	result.Month = q.Month
	result.Page = q.Page
	result.PageSize = q.PageSize
	report.SetMetadata(&result.Metadata, q)
	return &result, nil
}

func (r *usageLogRepository) PhotonthinxTrend(ctx context.Context, q report.Query) (*report.Trend, error) {
	format := "YYYY-MM-DD"
	if q.Granularity == "month" {
		format = "YYYY-MM"
	} else if q.Granularity != "day" {
		return nil, fmt.Errorf("invalid trend granularity")
	}
	userFilter := ""
	args := []any{q.Start, q.End, q.Now}
	if q.UserID > 0 {
		userFilter = " AND ul.user_id=$4"
		args = append(args, q.UserID)
	}
	// Keep the timestamp predicates indexable; timezone conversion is only for grouping.
	query := `WITH points AS (
 SELECT to_char(ul.created_at AT TIME ZONE 'Asia/Shanghai','` + format + `') AS date, ` + photonthinxUsageMetrics + `
 FROM usage_logs ul WHERE ul.created_at >= $1 AND ul.created_at < LEAST($2::timestamptz,$3::timestamptz)` + userFilter + `
 GROUP BY date
 ) SELECT json_build_object(
 'points',COALESCE((SELECT json_agg(p ORDER BY date) FROM points p),'[]'::json),
 'available_from',(SELECT created_at FROM usage_logs ORDER BY created_at ASC LIMIT 1))`
	var raw []byte
	if err := scanSingleRow(ctx, r.sql, query, args, &raw); err != nil {
		return nil, err
	}
	var result report.Trend
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
