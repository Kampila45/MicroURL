package application

import (
	"context"
	"fmt"
	"time"
	"url-shortener/internal/domain"
)

// GetLinkStatsUseCase loads aggregate redirect counts and zero-fills missing UTC dates
// over a 7-day or 30-day window.
type GetLinkStatsUseCase struct {
	repo LinkRepository
}

func NewGetLinkStatsUseCase(repo LinkRepository) *GetLinkStatsUseCase {
	return &GetLinkStatsUseCase{repo: repo}
}

type GetLinkStatsRequest struct {
	Slug string
	Days int
}

func (uc *GetLinkStatsUseCase) Execute(ctx context.Context, req GetLinkStatsRequest) (*domain.LinkStats, error) {
	days := req.Days
	if days != 7 && days != 30 {
		days = 7
	}

	nowUTC := time.Now().UTC()
	endDay := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	startDay := endDay.AddDate(0, 0, -(days - 1))

	stats, err := uc.repo.GetStats(ctx, req.Slug, startDay, days)
	if err != nil {
		return nil, fmt.Errorf("failed to load link stats: %w", err)
	}

	byDate := make(map[string]int64, len(stats.Daily))
	for _, entry := range stats.Daily {
		byDate[entry.Date] = entry.Clicks
	}

	continuous := make([]domain.DailyClickCount, 0, days)
	for i := 0; i < days; i++ {
		dayStr := startDay.AddDate(0, 0, i).Format("2006-01-02")
		continuous = append(continuous, domain.DailyClickCount{
			Date:   dayStr,
			Clicks: byDate[dayStr],
		})
	}

	stats.Timezone = "UTC"
	stats.RangeDays = days
	stats.Daily = continuous
	return stats, nil
}

