package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"smartcoop/internal/domain"
	"smartcoop/internal/timeutil"
)

// AnalyticsService contains period handling and input validation for analytics queries.
type AnalyticsService struct {
	repo domain.AnalyticsRepository
}

func NewAnalyticsService(repo domain.AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) GetTopSelling(ctx context.Context, period string, limit int) ([]domain.TopSellingProduct, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	period = strings.ToLower(strings.TrimSpace(period))
	if period == "" {
		period = "6m"
	}

	now := timeutil.Now()
	toYear, toMonth := now.Year(), int(now.Month())
	fromYear, fromMonth := 0, 0
	switch period {
	case "all":
		// Keep the lower bound open-ended. Future months are excluded by the upper bound.
	case "3m", "6m":
		months, _ := strconv.Atoi(strings.TrimSuffix(period, "m"))
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -(months - 1), 0)
		fromYear, fromMonth = start.Year(), int(start.Month())
	default:
		return nil, domain.NewValidationError("Periode harus berupa all, 3m, atau 6m")
	}
	return s.repo.GetTopSelling(ctx, fromYear, fromMonth, toYear, toMonth, limit)
}

func (s *AnalyticsService) GetTopSellingRange(ctx context.Context, from, to string, limit int) ([]domain.TopSellingProduct, error) {
	fromYear, fromMonth, err := parseYearMonth(from)
	if err != nil {
		return nil, domain.NewValidationError("Parameter from harus berformat YYYY-MM", domain.ValidationErrorDetail{Field: "from", Message: err.Error()})
	}
	toYear, toMonth, err := parseYearMonth(to)
	if err != nil {
		return nil, domain.NewValidationError("Parameter to harus berformat YYYY-MM", domain.ValidationErrorDetail{Field: "to", Message: err.Error()})
	}
	if fromYear*100+fromMonth > toYear*100+toMonth {
		return nil, domain.NewValidationError("Rentang from tidak boleh melewati to")
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	return s.repo.GetTopSelling(ctx, fromYear, fromMonth, toYear, toMonth, limit)
}

func (s *AnalyticsService) GetTopStock(ctx context.Context, limit int) ([]domain.TopStockProduct, int64, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	return s.repo.GetTopStock(ctx, limit)
}

func (s *AnalyticsService) GetSummary(ctx context.Context) (*domain.DashboardSummary, error) {
	return s.repo.GetSummary(ctx)
}

func (s *AnalyticsService) GetMonthlyTrend(ctx context.Context, productCode string) ([]domain.MonthlyTrendItem, error) {
	productCode = strings.TrimSpace(productCode)
	if productCode == "" {
		return nil, domain.NewValidationError("Parameter code wajib diisi", domain.ValidationErrorDetail{Field: "code", Message: "kode barang tidak boleh kosong"})
	}
	return s.repo.GetMonthlyTrend(ctx, productCode)
}

func parseYearMonth(value string) (year, month int, err error) {
	parsed, err := time.Parse("2006-01", strings.TrimSpace(value))
	if err != nil {
		return 0, 0, fmt.Errorf("nilai %q tidak valid", value)
	}
	return parsed.Year(), int(parsed.Month()), nil
}
