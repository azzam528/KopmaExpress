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

// GetTopSellingByPeriod menghitung barang terlaris untuk periode harian,
// mingguan, bulanan, atau tahunan.
//
//   - daily   : satu hari (date, default hari ini) dari pesanan COMPLETED.
//   - weekly  : minggu berjalan, Senin-Minggu, dari pesanan COMPLETED.
//   - monthly : satu bulan (month, default bulan ini) dari monthly_sales_history.
//   - yearly  : tahun berjalan dari monthly_sales_history.
//
// Harian/mingguan memakai pesanan langsung karena riwayat hasil import Excel
// hanya tersedia per bulan.
func (s *AnalyticsService) GetTopSellingByPeriod(
	ctx context.Context,
	period, date, month string,
	limit int,
) ([]domain.TopSellingProduct, *domain.TopSellingWindow, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	const dayLayout = "2006-01-02"
	period = strings.ToLower(strings.TrimSpace(period))
	date = strings.TrimSpace(date)
	month = strings.TrimSpace(month)

	loc := timeutil.Location()
	now := timeutil.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	switch period {
	case "daily":
		day := today
		if date != "" {
			parsed, err := time.ParseInLocation(dayLayout, date, loc)
			if err != nil {
				return nil, nil, domain.NewValidationError("Parameter date harus berformat YYYY-MM-DD",
					domain.ValidationErrorDetail{Field: "date", Message: fmt.Sprintf("nilai %q tidak valid", date)})
			}
			day = parsed
		}
		rows, err := s.repo.GetTopSellingOrders(ctx, day, day.AddDate(0, 0, 1), limit)
		if err != nil {
			return nil, nil, err
		}
		return rows, &domain.TopSellingWindow{
			Period: period, From: day.Format(dayLayout), To: day.Format(dayLayout),
		}, nil

	case "weekly":
		// Senin sebagai awal minggu (time.Weekday: Minggu=0).
		offset := (int(today.Weekday()) + 6) % 7
		start := today.AddDate(0, 0, -offset)
		end := start.AddDate(0, 0, 7)
		rows, err := s.repo.GetTopSellingOrders(ctx, start, end, limit)
		if err != nil {
			return nil, nil, err
		}
		return rows, &domain.TopSellingWindow{
			Period: period, From: start.Format(dayLayout), To: end.AddDate(0, 0, -1).Format(dayLayout),
		}, nil

	case "monthly":
		year, mon := now.Year(), int(now.Month())
		if month != "" {
			var err error
			year, mon, err = parseYearMonth(month)
			if err != nil {
				return nil, nil, domain.NewValidationError("Parameter month harus berformat YYYY-MM",
					domain.ValidationErrorDetail{Field: "month", Message: err.Error()})
			}
		}
		rows, err := s.repo.GetTopSelling(ctx, year, mon, year, mon, limit)
		if err != nil {
			return nil, nil, err
		}
		first := time.Date(year, time.Month(mon), 1, 0, 0, 0, 0, loc)
		return rows, &domain.TopSellingWindow{
			Period: period, From: first.Format(dayLayout), To: first.AddDate(0, 1, -1).Format(dayLayout),
		}, nil

	case "yearly":
		year := now.Year()
		rows, err := s.repo.GetTopSelling(ctx, year, 1, year, 12, limit)
		if err != nil {
			return nil, nil, err
		}
		return rows, &domain.TopSellingWindow{
			Period: period, From: fmt.Sprintf("%04d-01-01", year), To: fmt.Sprintf("%04d-12-31", year),
		}, nil
	}

	return nil, nil, domain.NewValidationError("Periode harus berupa daily, weekly, monthly, atau yearly")
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
