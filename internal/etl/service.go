package etl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"smartcoop/internal/domain"
	"smartcoop/internal/etl/excelgen"
)

const (
	StockModeReplace = "replace"
	StockModeSkip    = "skip"

	defaultBatchSize = 500
	importTimeout    = 10 * time.Minute
)

var (
	ErrInvalidStockMode = errors.New("stock_mode harus 'replace' atau 'skip'")
	ErrInvalidWorkbook  = errors.New("file bukan Excel .xlsx yang valid")
	ErrMissingSheet     = fmt.Errorf("sheet '%s' tidak ditemukan", SheetProducts)
)

var _ domain.ETLService = (*Service)(nil)

type Service struct {
	products  domain.ProductRepository
	history   domain.HistoryRepository
	jobs      domain.ImportJobRepository
	batchSize int
}

func NewService(products domain.ProductRepository, history domain.HistoryRepository, jobs domain.ImportJobRepository) *Service {
	return &Service{products: products, history: history, jobs: jobs, batchSize: defaultBatchSize}
}

// StartImport validates the upload, registers a RUNNING job and returns at once; the import itself runs in the background.
func (s *Service) StartImport(ctx context.Context, fileName string, fileBytes []byte, stockMode string) (*domain.ImportJob, error) {
	mode := strings.ToLower(strings.TrimSpace(stockMode))
	if mode == "" {
		mode = StockModeReplace
	}
	if mode != StockModeReplace && mode != StockModeSkip {
		return nil, ErrInvalidStockMode
	}

	wb, err := OpenWorkbook(fileBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWorkbook, err)
	}
	if _, ok := FindProductSheet(wb); !ok {
		_ = wb.Close()
		return nil, ErrMissingSheet
	}

	job := &domain.ImportJob{
		ID:        primitive.NewObjectID(),
		FileName:  fileName,
		Status:    domain.ImportStatusRunning,
		Errors:    []domain.RowError{},
		StartedAt: time.Now(),
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		_ = wb.Close()
		return nil, err
	}

	go s.run(job.ID, wb, mode)
	return job, nil
}

func (s *Service) GetJob(ctx context.Context, id primitive.ObjectID) (*domain.ImportJob, error) {
	return s.jobs.FindByID(ctx, id)
}

func (s *Service) ListJobs(ctx context.Context, page, limit int) ([]domain.ImportJob, int64, error) {
	return s.jobs.FindAll(ctx, page, limit)
}

func (s *Service) GenerateDummyExcel(items, months int, dirty bool, seed int64) ([]byte, error) {
	f, err := excelgen.Generate(excelgen.Options{Items: items, Months: months, Dirty: dirty, Seed: seed})
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Service) run(id primitive.ObjectID, wb *excelize.File, stockMode string) {
	defer wb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), importTimeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			_ = s.jobs.MarkFailed(ctx, id, fmt.Sprintf("import berhenti tiba-tiba: %v", r))
		}
	}()

	if err := s.execute(ctx, id, wb, stockMode); err != nil {
		_ = s.jobs.MarkFailed(ctx, id, err.Error())
	}
}

type tally struct {
	processed, inserted, updated, skipped, failed int64
}

// settle books a written batch: rows the database neither inserted nor updated count as skipped.
func (t *tally) settle(batch int, inserted, updated int64) {
	t.processed += int64(batch)
	t.inserted += inserted
	t.updated += updated
	if rest := int64(batch) - inserted - updated; rest > 0 {
		t.skipped += rest
	}
}

func (s *Service) execute(ctx context.Context, id primitive.ObjectID, wb *excelize.File, stockMode string) error {
	known := s.knownCodeLookup(ctx)
	res, err := ParseWorkbookWith(wb, ParseOptions{KnownCode: known})
	if err != nil {
		return err
	}

	products := DedupeProducts(res.Products)
	history := DedupeHistory(res.History)
	duplicates := int64(len(res.Products)-len(products)) + int64(len(res.History)-len(history))

	total := int64(len(res.Products)+len(res.History)) + res.Failed
	t := tally{failed: res.Failed, skipped: duplicates, processed: res.Failed + duplicates}

	rowErrors := res.Errors
	if rowErrors == nil {
		rowErrors = []domain.RowError{}
	}
	report := func() error {
		return s.jobs.UpdateProgress(ctx, id, t.processed, t.inserted, t.updated, t.skipped, t.failed, rowErrors)
	}
	if err := report(); err != nil {
		return err
	}

	for start := 0; start < len(products); start += s.batchSize {
		batch := products[start:min(start+s.batchSize, len(products))]
		inserted, updated, err := s.products.BulkUpsert(ctx, batch, stockMode)
		if err != nil {
			return fmt.Errorf("simpan barang: %w", err)
		}
		t.settle(len(batch), inserted, updated)
		if err := report(); err != nil {
			return err
		}
	}

	for start := 0; start < len(history); start += s.batchSize {
		batch := history[start:min(start+s.batchSize, len(history))]
		inserted, updated, err := s.history.BulkUpsert(ctx, batch)
		if err != nil {
			return fmt.Errorf("simpan riwayat penjualan: %w", err)
		}
		t.settle(len(batch), inserted, updated)
		if err := report(); err != nil {
			return err
		}
	}

	return s.jobs.MarkCompleted(ctx, id, total)
}

// knownCodeLookup answers "does this code exist in the database" and remembers answers,
// so a code repeated across many history rows costs one query.
func (s *Service) knownCodeLookup(ctx context.Context) func(string) bool {
	cache := map[string]bool{}
	return func(code string) bool {
		if ok, seen := cache[code]; seen {
			return ok
		}
		p, err := s.products.FindByCode(ctx, code)
		ok := err == nil && p != nil
		cache[code] = ok
		return ok
	}
}
