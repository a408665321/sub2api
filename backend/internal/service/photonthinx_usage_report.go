package service

import (
	"context"
	"fmt"
	report "github.com/Wei-Shaw/sub2api/internal/photonthinx/usagereport"
	"time"
)

// The optional report interface keeps upstream repositories and DI unchanged.
func (s *UsageService) PhotonthinxUsageMonthly(ctx context.Context, q report.Query) (*report.Monthly, error) {
	repo, ok := s.usageRepo.(report.Repository)
	if !ok {
		return nil, fmt.Errorf("Photonthinx usage report repository unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return repo.PhotonthinxMonthly(ctx, q)
}

func (s *UsageService) PhotonthinxUsageTrend(ctx context.Context, q report.Query) (*report.Trend, error) {
	repo, ok := s.usageRepo.(report.Repository)
	if !ok {
		return nil, fmt.Errorf("Photonthinx usage report repository unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := repo.PhotonthinxTrend(ctx, q)
	if err != nil {
		return nil, err
	}
	report.FillTrend(result, q)
	return result, nil
}
