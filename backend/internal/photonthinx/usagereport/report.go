// Package usagereport defines the independent Photonthinx retained-usage report contract.
package usagereport

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

var Shanghai = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}()

type Query struct {
	Month                            string
	Start, End, Now                  time.Time
	Page, PageSize                   int
	Search, Sort, Order, Granularity string
	UserID                           int64
}

// Parse bounds query cost and keeps browser/session time zones out of calendar logic.
func Parse(v url.Values, now time.Time, trend bool) (Query, error) {
	q := Query{Month: v.Get("month"), Now: now.In(Shanghai), Page: 1, PageSize: 20, Search: strings.TrimSpace(v.Get("search")), Sort: v.Get("sort_by"), Order: v.Get("sort_order"), Granularity: v.Get("granularity")}
	if q.Month == "" {
		q.Month = q.Now.Format("2006-01")
	}
	start, err := time.ParseInLocation("2006-01", q.Month, Shanghai)
	if err != nil || start.Year() < 2000 || q.Month > q.Now.Format("2006-01") {
		return q, fmt.Errorf("month must be YYYY-MM between 2000-01 and the current Shanghai month")
	}
	q.Start = start
	q.End = start.AddDate(0, 1, 0)
	integer := func(key string, fallback, min, max int) (int, error) {
		if v.Get(key) == "" {
			return fallback, nil
		}
		n, e := strconv.Atoi(v.Get(key))
		if e != nil || n < min || n > max {
			return 0, fmt.Errorf("invalid %s (%d..%d)", key, min, max)
		}
		return n, nil
	}
	if q.Page, err = integer("page", 1, 1, 1000000); err != nil {
		return q, err
	}
	if q.PageSize, err = integer("page_size", 20, 1, 100); err != nil {
		return q, err
	}
	if len(q.Search) > 200 {
		return q, fmt.Errorf("search is too long")
	}
	if q.Sort == "" {
		q.Sort = "actual_cost"
	}
	if q.Sort != "actual_cost" && q.Sort != "requests" && q.Sort != "total_tokens" {
		return q, fmt.Errorf("invalid sort_by")
	}
	if q.Order == "" {
		q.Order = "desc"
	}
	if q.Order != "asc" && q.Order != "desc" {
		return q, fmt.Errorf("invalid sort_order")
	}
	if q.Granularity == "" {
		q.Granularity = "day"
	}
	if q.Granularity != "day" && q.Granularity != "month" {
		return q, fmt.Errorf("invalid granularity")
	}
	if raw := v.Get("user_id"); raw != "" {
		q.UserID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || q.UserID <= 0 {
			return q, fmt.Errorf("invalid user_id")
		}
	}
	if trend {
		fallback := 1
		if q.Granularity == "month" {
			fallback = 12
		}
		months, e := integer("months", fallback, 1, 12)
		if e != nil {
			return q, e
		}
		if q.Granularity == "day" && months != 1 {
			return q, fmt.Errorf("daily trend is limited to one month")
		}
		q.Start = q.Start.AddDate(0, 1-months, 0)
	}
	return q, nil
}

type Metrics struct {
	ActualCost          float64 `json:"actual_cost"`
	AccountCost         float64 `json:"account_cost"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
}
type Summary struct {
	Metrics
	ActiveUsers int64 `json:"active_users"`
}
type User struct {
	Metrics
	UserID    int64   `json:"user_id"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	CostShare float64 `json:"cost_share"`
}
type Metadata struct {
	AvailableFrom     *time.Time `json:"available_from"`
	HistoryIncomplete bool       `json:"history_incomplete"`
	Current           bool       `json:"current"`
	AsOf              time.Time  `json:"as_of"`
	Timezone          string     `json:"timezone"`
}
type Monthly struct {
	Metadata
	Month    string  `json:"month"`
	Summary  Summary `json:"summary"`
	Items    []User  `json:"items"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}
type Point struct {
	Metrics
	Date      string `json:"date"`
	Available bool   `json:"available"`
	Partial   bool   `json:"partial"`
}
type Trend struct {
	Metadata
	Points []Point `json:"points"`
}
type Repository interface {
	PhotonthinxMonthly(context.Context, Query) (*Monthly, error)
	PhotonthinxTrend(context.Context, Query) (*Trend, error)
}

func SetMetadata(m *Metadata, q Query) {
	m.Timezone = "Asia/Shanghai"
	m.AsOf = q.Now
	m.Current = q.Month == q.Now.Format("2006-01")
	m.HistoryIncomplete = m.AvailableFrom == nil || m.AvailableFrom.After(q.Start)
}

// Missing retained history is explicitly unavailable; only known-range gaps become zero.
func FillTrend(result *Trend, q Query) {
	SetMetadata(&result.Metadata, q)
	points := make(map[string]Point, len(result.Points))
	for _, p := range result.Points {
		points[p.Date] = p
	}
	result.Points = make([]Point, 0, 31)
	for day := q.Start; day.Before(q.End) && !day.After(q.Now); {
		next := day.AddDate(0, 0, 1)
		format := "2006-01-02"
		if q.Granularity == "month" {
			next = day.AddDate(0, 1, 0)
			format = "2006-01"
		}
		key := day.Format(format)
		p := points[key]
		p.Date = key
		p.Available = result.AvailableFrom != nil && result.AvailableFrom.Before(next)
		p.Partial = p.Available && (result.AvailableFrom.After(day) || next.After(q.Now))
		result.Points = append(result.Points, p)
		day = next
	}
}
