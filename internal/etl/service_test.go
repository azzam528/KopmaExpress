package etl

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"smartcoop/internal/domain"
)

type fakeProducts struct {
	domain.ProductRepository
	data map[string]domain.Product
}

func (f *fakeProducts) FindByCode(_ context.Context, code string) (*domain.Product, error) {
	if p, ok := f.data[code]; ok {
		return &p, nil
	}
	return nil, nil
}

func (f *fakeProducts) BulkUpsert(_ context.Context, products []domain.Product, mode string) (int64, int64, error) {
	var inserted, updated int64
	for _, p := range products {
		old, exists := f.data[p.Code]
		if !exists {
			inserted++
		} else {
			updated++
			if mode == StockModeSkip {
				p.Stock = old.Stock
			}
		}
		f.data[p.Code] = p
	}
	return inserted, updated, nil
}

type fakeHistory struct {
	domain.HistoryRepository
	data map[string]domain.MonthlySalesHistory
}

func (f *fakeHistory) BulkUpsert(_ context.Context, records []domain.MonthlySalesHistory) (int64, int64, error) {
	var inserted, updated int64
	for _, r := range records {
		key := fmt.Sprintf("%s-%d-%d", r.ProductCode, r.Year, r.Month)
		if _, exists := f.data[key]; exists {
			updated++
		} else {
			inserted++
		}
		f.data[key] = r
	}
	return inserted, updated, nil
}

type fakeJobs struct {
	domain.ImportJobRepository
	last      domain.ImportJob
	total     int64
	completed bool
}

func (f *fakeJobs) UpdateProgress(_ context.Context, _ primitive.ObjectID, processed, inserted, updated, skipped, failed int64, errs []domain.RowError) error {
	f.last = domain.ImportJob{
		ProcessedRows: processed,
		Inserted:      inserted,
		Updated:       updated,
		Skipped:       skipped,
		Failed:        failed,
		Errors:        errs,
	}
	return nil
}

func (f *fakeJobs) MarkCompleted(_ context.Context, _ primitive.ObjectID, total int64) error {
	f.total = total
	f.completed = true
	return nil
}

func newFakeService() (*Service, *fakeProducts, *fakeHistory, *fakeJobs) {
	products := &fakeProducts{data: map[string]domain.Product{}}
	history := &fakeHistory{data: map[string]domain.MonthlySalesHistory{}}
	jobs := &fakeJobs{}
	return NewService(products, history, jobs), products, history, jobs
}

func TestExecuteCountsRows(t *testing.T) {
	svc, products, history, jobs := newFakeService()

	if err := svc.execute(context.Background(), primitive.NewObjectID(), buildWorkbook(t), StockModeReplace); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Workbook: 3 product rows (one duplicate code), 2 history rows, 2 rows that fail cleaning.
	if len(products.data) != 2 || len(history.data) != 2 {
		t.Errorf("tersimpan %d barang dan %d riwayat, want 2 dan 2", len(products.data), len(history.data))
	}
	got := jobs.last
	if got.Inserted != 4 || got.Updated != 0 || got.Skipped != 1 || got.Failed != 2 {
		t.Errorf("job = inserted %d, updated %d, skipped %d, failed %d; want 4, 0, 1, 2", got.Inserted, got.Updated, got.Skipped, got.Failed)
	}
	if !jobs.completed || jobs.total != 7 || got.ProcessedRows != jobs.total {
		t.Errorf("completed = %v, total = %d, processed = %d; want true, 7, 7", jobs.completed, jobs.total, got.ProcessedRows)
	}
	if len(got.Errors) != 2 {
		t.Errorf("errors = %d, want 2", len(got.Errors))
	}
}

func TestExecuteIsIdempotent(t *testing.T) {
	svc, products, history, _ := newFakeService()
	run := func() *fakeJobs {
		jobs := &fakeJobs{}
		svc.jobs = jobs
		if err := svc.execute(context.Background(), primitive.NewObjectID(), buildWorkbook(t), StockModeReplace); err != nil {
			t.Fatalf("execute: %v", err)
		}
		return jobs
	}

	run()
	second := run()

	if len(products.data) != 2 || len(history.data) != 2 {
		t.Errorf("import kedua mengubah jumlah data: %d barang, %d riwayat", len(products.data), len(history.data))
	}
	if got := products.data["BRG-001"].Stock; got != 10 {
		t.Errorf("stok BRG-001 = %d, want 10 (replace, bukan dijumlahkan)", got)
	}
	if second.last.Inserted != 0 || second.last.Updated != 4 {
		t.Errorf("import kedua: inserted %d, updated %d; want 0 dan 4", second.last.Inserted, second.last.Updated)
	}
}

func TestExecuteSkipModeKeepsStock(t *testing.T) {
	svc, products, _, _ := newFakeService()
	products.data["BRG-002"] = domain.Product{Code: "BRG-002", Name: "Pensil", Stock: 999}

	if err := svc.execute(context.Background(), primitive.NewObjectID(), buildWorkbook(t), StockModeSkip); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := products.data["BRG-002"].Stock; got != 999 {
		t.Errorf("stok BRG-002 = %d, want 999 (mode skip)", got)
	}
}

func TestStartImportValidation(t *testing.T) {
	svc := NewService(nil, nil, nil)

	if _, err := svc.StartImport(context.Background(), "a.xlsx", nil, "tambah"); !errors.Is(err, ErrInvalidStockMode) {
		t.Errorf("stock_mode salah: err = %v", err)
	}
	if _, err := svc.StartImport(context.Background(), "a.xlsx", []byte("bukan excel"), ""); !errors.Is(err, ErrInvalidWorkbook) {
		t.Errorf("file rusak: err = %v", err)
	}

	empty := excelize.NewFile()
	buf, err := empty.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartImport(context.Background(), "a.xlsx", buf.Bytes(), ""); !errors.Is(err, ErrMissingSheet) {
		t.Errorf("tanpa sheet Barang: err = %v", err)
	}
}