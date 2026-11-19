package service

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"smartcoop/internal/domain"
	"smartcoop/internal/forecast"
	"smartcoop/internal/timeutil"
)

// ForecastService orchestrates series preparation, worker-pool prediction,
// global restock classification, and persistence.
type ForecastService struct {
	repo         domain.ForecastRepository
	workers      int
	topStockSize int

	mu      sync.Mutex
	lastRun time.Time
}

func NewForecastService(repo domain.ForecastRepository, workers, topStockSize int) *ForecastService {
	if workers < 1 {
		workers = runtime.NumCPU()
	}
	if topStockSize < 1 {
		topStockSize = 10
	}
	return &ForecastService{repo: repo, workers: workers, topStockSize: topStockSize}
}

func (s *ForecastService) Recalculate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	now := timeutil.Now()
	targetMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01")
	products, err := s.repo.LoadProductSales(ctx)
	if err != nil {
		return fmt.Errorf("load product sales for forecast: %w", err)
	}
	if len(products) == 0 {
		return s.repo.UpsertForecasts(ctx, []domain.ForecastResult{})
	}

	workerCount := s.workers
	if workerCount > len(products) {
		workerCount = len(products)
	}
	jobs := make(chan domain.ProductSalesData, len(products))
	results := make(chan domain.ForecastResult, len(products))

	var workerGroup sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			for productSales := range jobs {
				if ctx.Err() != nil {
					continue
				}
				series := forecast.BuildSeries(toMonthKeyMap(productSales.Sales), now, 12)
				prediction := forecast.Predict(series)
				results <- domain.ForecastResult{
					ProductCode:     productSales.Product.Code,
					ProductName:     productSales.Product.Name,
					TargetMonth:     targetMonth,
					CurrentStock:    productSales.Product.Stock,
					WMA:             prediction.WMA,
					LinearTrend:     prediction.Linear,
					Slope:           prediction.Slope,
					R2:              prediction.R2,
					PredictedDemand: prediction.Predicted,
					DataPoints:      prediction.DataPoints,
					Status:          domain.StatusSafe,
					CalculatedAt:    now,
				}
			}
		}()
	}

	for _, product := range products {
		jobs <- product
	}
	close(jobs)
	go func() {
		workerGroup.Wait()
		close(results)
	}()

	resultsList := make([]domain.ForecastResult, 0, len(products))
	for result := range results {
		resultsList = append(resultsList, result)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(resultsList) != len(products) {
		return fmt.Errorf("forecast worker pool returned %d results for %d products", len(resultsList), len(products))
	}

	assignStatus(resultsList, s.topStockSize)
	if err := s.repo.UpsertForecasts(ctx, resultsList); err != nil {
		return fmt.Errorf("persist forecast results: %w", err)
	}
	return nil
}

// EnsureFresh menghitung ulang forecast jika hasil terakhir lebih tua dari maxAge.
// Mutex membuat request bersamaan (summary + list dari dashboard) hanya memicu satu hitungan.
func (s *ForecastService) EnsureFresh(ctx context.Context, maxAge time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.lastRun.IsZero() && time.Since(s.lastRun) < maxAge {
		return nil
	}
	if err := s.Recalculate(ctx); err != nil {
		return err
	}
	s.lastRun = time.Now()
	return nil
}

func (s *ForecastService) List(ctx context.Context, filter domain.ForecastFilter) ([]domain.ForecastResult, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Status != "" {
		switch filter.Status {
		case domain.StatusCritical, domain.StatusOverstock, domain.StatusSafe, domain.StatusInsufficientData:
		default:
			return nil, 0, domain.NewValidationError("Status forecast tidak dikenali")
		}
	}
	return s.repo.FindAll(ctx, filter)
}

func (s *ForecastService) GetByProductCode(ctx context.Context, code string) (*domain.ForecastDetail, error) {
	if code == "" {
		return nil, domain.NewValidationError("Kode barang wajib diisi")
	}
	result, err := s.repo.FindByProductCode(ctx, code)
	if err != nil {
		return nil, err
	}
	history, err := s.repo.FindSalesHistory(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("load monthly history for %s: %w", code, err)
	}
	return &domain.ForecastDetail{ForecastResult: *result, History: history}, nil
}

func (s *ForecastService) GetSummary(ctx context.Context) (map[domain.RestockStatus]int64, error) {
	return s.repo.GetStatusSummary(ctx)
}

func toMonthKeyMap(sales map[string]int64) map[forecast.MonthKey]int64 {
	result := make(map[forecast.MonthKey]int64, len(sales))
	for key, quantity := range sales {
		month, err := time.Parse("2006-01", key)
		if err != nil || quantity < 0 {
			continue
		}
		result[forecast.MonthKey{Year: month.Year(), Month: int(month.Month())}] += quantity
	}
	return result
}

func assignStatus(results []domain.ForecastResult, topN int) {
	if len(results) == 0 {
		return
	}
	if topN < 1 {
		topN = 10
	}

	predictions := make([]int64, 0, len(results))
	for _, result := range results {
		predictions = append(predictions, result.PredictedDemand)
	}
	sort.Slice(predictions, func(i, j int) bool { return predictions[i] < predictions[j] })
	median := predictions[len(predictions)/2] // upper median, matching PRD §7.8

	indices := make([]int, len(results))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		left, right := results[indices[i]], results[indices[j]]
		if left.CurrentStock == right.CurrentStock {
			return left.ProductCode < right.ProductCode
		}
		return left.CurrentStock > right.CurrentStock
	})

	topStock := make(map[int]struct{}, topN)
	for i := 0; i < topN && i < len(indices); i++ {
		topStock[indices[i]] = struct{}{}
	}

	for i := range results {
		result := &results[i]
		result.RestockQty = 0
		switch {
		case result.DataPoints < 3:
			result.Status = domain.StatusInsufficientData
		case result.CurrentStock < result.PredictedDemand:
			result.Status = domain.StatusCritical
			result.RestockQty = result.PredictedDemand - result.CurrentStock
		default:
			_, isTopStock := topStock[i]
			lowDemand := float64(result.PredictedDemand) < 0.5*float64(median)
			veryHighStock := result.CurrentStock >= 3*maxInt64(result.PredictedDemand, 1)
			if isTopStock && (lowDemand || veryHighStock) {
				result.Status = domain.StatusOverstock
			} else {
				result.Status = domain.StatusSafe
			}
		}
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
