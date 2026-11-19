package service

import (
	"context"
	"testing"
	"time"

	"smartcoop/internal/domain"
)

type fakeAnalyticsRepo struct {
	// pemanggilan GetTopSelling (riwayat bulanan)
	historyCalls [][4]int
	// pemanggilan GetTopSellingOrders (pesanan langsung)
	orderFrom, orderTo time.Time
	orderCalled        bool
}

func (f *fakeAnalyticsRepo) GetTopSelling(_ context.Context, fy, fm, ty, tm, _ int) ([]domain.TopSellingProduct, error) {
	f.historyCalls = append(f.historyCalls, [4]int{fy, fm, ty, tm})
	return []domain.TopSellingProduct{}, nil
}

func (f *fakeAnalyticsRepo) GetTopSellingOrders(_ context.Context, from, to time.Time, _ int) ([]domain.TopSellingProduct, error) {
	f.orderCalled = true
	f.orderFrom, f.orderTo = from, to
	return []domain.TopSellingProduct{}, nil
}

func (f *fakeAnalyticsRepo) GetTopStock(context.Context, int) ([]domain.TopStockProduct, int64, error) {
	return nil, 0, nil
}

func (f *fakeAnalyticsRepo) GetSummary(context.Context) (*domain.DashboardSummary, error) {
	return &domain.DashboardSummary{}, nil
}

func (f *fakeAnalyticsRepo) GetMonthlyTrend(context.Context, string) ([]domain.MonthlyTrendItem, error) {
	return nil, nil
}

func TestGetTopSellingByPeriodDailyUsesOrdersForChosenDate(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	svc := NewAnalyticsService(repo)

	_, window, err := svc.GetTopSellingByPeriod(context.Background(), "daily", "2026-11-17", "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.orderCalled {
		t.Fatal("daily harus membaca dari pesanan")
	}
	if got := repo.orderTo.Sub(repo.orderFrom); got != 24*time.Hour {
		t.Fatalf("rentang harian harus 24 jam, got %v", got)
	}
	if window.From != "2026-11-17" || window.To != "2026-11-17" {
		t.Fatalf("window salah: %+v", window)
	}
}

func TestGetTopSellingByPeriodWeeklyRunsMondayToSunday(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	svc := NewAnalyticsService(repo)

	_, window, err := svc.GetTopSellingByPeriod(context.Background(), "weekly", "", "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	from, _ := time.Parse("2006-01-02", window.From)
	to, _ := time.Parse("2006-01-02", window.To)
	if from.Weekday() != time.Monday {
		t.Fatalf("minggu harus dimulai Senin, got %v", from.Weekday())
	}
	if to.Sub(from) != 6*24*time.Hour {
		t.Fatalf("minggu harus 7 hari (Senin-Minggu): %s - %s", window.From, window.To)
	}
}

func TestGetTopSellingByPeriodMonthlyUsesChosenMonth(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	svc := NewAnalyticsService(repo)

	_, window, err := svc.GetTopSellingByPeriod(context.Background(), "monthly", "", "2026-02", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.historyCalls) != 1 || repo.historyCalls[0] != [4]int{2026, 2, 2026, 2} {
		t.Fatalf("pemanggilan repo salah: %+v", repo.historyCalls)
	}
	if window.From != "2026-02-01" || window.To != "2026-02-28" {
		t.Fatalf("window salah: %+v", window)
	}
}

func TestGetTopSellingByPeriodYearlyCoversWholeYear(t *testing.T) {
	repo := &fakeAnalyticsRepo{}
	svc := NewAnalyticsService(repo)

	if _, _, err := svc.GetTopSellingByPeriod(context.Background(), "yearly", "", "", 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := repo.historyCalls[0]
	if call[0] != call[2] || call[1] != 1 || call[3] != 12 {
		t.Fatalf("yearly harus Januari-Desember pada tahun yang sama: %+v", call)
	}
}

func TestGetTopSellingByPeriodRejectsInvalidInput(t *testing.T) {
	svc := NewAnalyticsService(&fakeAnalyticsRepo{})
	cases := []struct{ name, period, date, month string }{
		{"periode tidak dikenal", "decade", "", ""},
		{"tanggal salah format", "daily", "17-11-2026", ""},
		{"bulan salah format", "monthly", "", "2026/02"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := svc.GetTopSellingByPeriod(context.Background(), tc.period, tc.date, tc.month, 10); err == nil {
				t.Fatal("seharusnya error validasi")
			}
		})
	}
}
