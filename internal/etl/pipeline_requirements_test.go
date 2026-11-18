package etl_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"smartcoop/internal/domain"
	"smartcoop/internal/etl"
)

// Mock repos for pipeline tests
type mockProductRepo struct {
	domain.ProductRepository
	products   map[string]domain.Product
	upsertErr  error
	upsertHook func(products []domain.Product)
}

func newMockProductRepo() *mockProductRepo {
	return &mockProductRepo{
		products: make(map[string]domain.Product),
	}
}

func (m *mockProductRepo) FindByCode(_ context.Context, code string) (*domain.Product, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if p, ok := m.products[code]; ok {
		return &p, nil
	}
	return nil, domain.NewNotFoundError("Produk tidak ditemukan")
}

func (m *mockProductRepo) FindAll(_ context.Context, filter domain.ProductFilter) ([]domain.Product, int64, error) {
	var list []domain.Product
	for _, p := range m.products {
		if filter.IsActive != nil && p.IsActive != *filter.IsActive {
			continue
		}
		if filter.Query != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(filter.Query)) &&
			!strings.Contains(strings.ToLower(p.Code), strings.ToLower(filter.Query)) {
			continue
		}
		list = append(list, p)
	}
	return list, int64(len(list)), nil
}

func (m *mockProductRepo) BulkUpsert(_ context.Context, products []domain.Product, stockMode string) (int64, int64, error) {
	if m.upsertErr != nil {
		return 0, 0, m.upsertErr
	}
	if m.upsertHook != nil {
		m.upsertHook(products)
	}
	var inserted, updated int64
	for _, p := range products {
		code := strings.ToUpper(strings.TrimSpace(p.Code))
		existing, ok := m.products[code]
		if !ok {
			inserted++
			p.IsActive = true
			m.products[code] = p
		} else {
			updated++
			if stockMode == etl.StockModeSkip {
				p.Stock = existing.Stock
			}
			p.IsActive = true
			m.products[code] = p
		}
	}
	return inserted, updated, nil
}

type mockHistoryRepo struct {
	domain.HistoryRepository
	history map[string]domain.MonthlySalesHistory
}

func newMockHistoryRepo() *mockHistoryRepo {
	return &mockHistoryRepo{history: make(map[string]domain.MonthlySalesHistory)}
}

func (m *mockHistoryRepo) BulkUpsert(_ context.Context, records []domain.MonthlySalesHistory) (int64, int64, error) {
	var inserted, updated int64
	for _, r := range records {
		key := r.ProductCode + "-" + string(rune(r.Year)) + "-" + string(rune(r.Month))
		if _, ok := m.history[key]; !ok {
			inserted++
		} else {
			updated++
		}
		m.history[key] = r
	}
	return inserted, updated, nil
}

type mockJobRepo struct {
	domain.ImportJobRepository
	jobs           map[primitive.ObjectID]*domain.ImportJob
	completedOrder []string
}

func newMockJobRepo() *mockJobRepo {
	return &mockJobRepo{jobs: make(map[primitive.ObjectID]*domain.ImportJob)}
}

func (m *mockJobRepo) Create(_ context.Context, job *domain.ImportJob) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobRepo) FindByID(_ context.Context, id primitive.ObjectID) (*domain.ImportJob, error) {
	if j, ok := m.jobs[id]; ok {
		return j, nil
	}
	return nil, errors.New("not found")
}

func (m *mockJobRepo) UpdateProgress(_ context.Context, id primitive.ObjectID, processed, inserted, updated, skipped, failed int64, errs []domain.RowError) error {
	if j, ok := m.jobs[id]; ok {
		j.ProcessedRows = processed
		j.Inserted = inserted
		j.Updated = updated
		j.Skipped = skipped
		j.Failed = failed
		j.Errors = errs
	}
	return nil
}

func (m *mockJobRepo) MarkCompleted(_ context.Context, id primitive.ObjectID, totalRows int64) error {
	if j, ok := m.jobs[id]; ok {
		now := time.Now()
		j.Status = domain.ImportStatusDone
		j.TotalRows = totalRows
		j.FinishedAt = &now
		m.completedOrder = append(m.completedOrder, "MarkCompleted")
	}
	return nil
}

func (m *mockJobRepo) MarkFailed(_ context.Context, id primitive.ObjectID, reason string) error {
	if j, ok := m.jobs[id]; ok {
		now := time.Now()
		j.Status = domain.ImportStatusFailed
		j.FinishedAt = &now
		j.Errors = append(j.Errors, domain.RowError{Sheet: "SYSTEM", Row: 0, Reason: reason})
		m.completedOrder = append(m.completedOrder, "MarkFailed")
	}
	return nil
}

// Helper to create an Excel file in-memory
func makeTestExcel(t *testing.T, sheetName string, rows [][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	if sheetName != "Sheet1" {
		_ = f.SetSheetName("Sheet1", sheetName)
	}
	for i, r := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			t.Fatal(err)
		}
		_ = f.SetSheetRow(sheetName, cell, &r)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// -------------------------------------------------------------------------
// TEST 1: Excel valid berhasil menghasilkan data produk
// -------------------------------------------------------------------------
func TestReq1_ValidExcelParsesProducts(t *testing.T) {
	data := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"BRG-001", "Buku Tulis", "ATK", 5000, 20},
		{"BRG-002", "Pensil 2B", "ATK", 2000, 50},
	})

	wb, err := etl.OpenWorkbook(data)
	if err != nil {
		t.Fatalf("OpenWorkbook error: %v", err)
	}
	defer wb.Close()

	res, err := etl.ParseWorkbook(wb)
	if err != nil {
		t.Fatalf("ParseWorkbook error: %v", err)
	}

	if len(res.Products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(res.Products))
	}
	if res.Products[0].Code != "BRG-001" || res.Products[0].Stock != 20 {
		t.Errorf("product 1 mismatch: %+v", res.Products[0])
	}
	if res.Products[1].Code != "BRG-002" || res.Products[1].Stock != 50 {
		t.Errorf("product 2 mismatch: %+v", res.Products[1])
	}
}

// -------------------------------------------------------------------------
// TEST 2: Produk baru berhasil masuk ke collection products
// -------------------------------------------------------------------------
func TestReq2_NewProductsEnterProductsCollection(t *testing.T) {
	pRepo := newMockProductRepo()
	hRepo := newMockHistoryRepo()
	jRepo := newMockJobRepo()
	svc := etl.NewService(pRepo, hRepo, jRepo)

	data := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"NEW-001", "Kopi Susu", "Minuman", 3500, 15},
		{"NEW-002", "Roti Bakar", "Makanan", 7000, 10},
	})

	job, err := svc.StartImport(context.Background(), "test.xlsx", data, "replace")
	if err != nil {
		t.Fatalf("StartImport error: %v", err)
	}

	// Wait for background job to finish
	time.Sleep(100 * time.Millisecond)

	curJob, _ := jRepo.FindByID(context.Background(), job.ID)
	if curJob.Status != domain.ImportStatusDone {
		t.Fatalf("expected DONE, got %s", curJob.Status)
	}
	if curJob.Inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", curJob.Inserted)
	}

	p1, err := pRepo.FindByCode(context.Background(), "NEW-001")
	if err != nil || p1.Name != "Kopi Susu" || p1.Stock != 15 {
		t.Errorf("NEW-001 not saved properly in products: %+v, err: %v", p1, err)
	}
	p2, err := pRepo.FindByCode(context.Background(), "NEW-002")
	if err != nil || p2.Name != "Roti Bakar" || p2.Stock != 10 {
		t.Errorf("NEW-002 not saved properly in products: %+v, err: %v", p2, err)
	}
}

// -------------------------------------------------------------------------
// TEST 3: Produk dengan kode yang sama diperbarui sesuai aturan upsert (replace vs skip)
// -------------------------------------------------------------------------
func TestReq3_UpsertRulesReplaceAndSkip(t *testing.T) {
	// 3A: Replace mode
	pRepo := newMockProductRepo()
	pRepo.products["BRG-EXIST"] = domain.Product{
		Code: "BRG-EXIST", Name: "Barang Lama", Category: "ATK", Price: 1000, Stock: 50, IsActive: true,
	}
	hRepo := newMockHistoryRepo()
	jRepo := newMockJobRepo()
	svc := etl.NewService(pRepo, hRepo, jRepo)

	dataReplace := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"BRG-EXIST", "Barang Baru", "Elektronik", 5000, 10},
	})

	_, err := svc.StartImport(context.Background(), "replace.xlsx", dataReplace, "replace")
	if err != nil {
		t.Fatalf("StartImport replace error: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	p, _ := pRepo.FindByCode(context.Background(), "BRG-EXIST")
	if p.Name != "Barang Baru" || p.Stock != 10 || p.Price != 5000 {
		t.Errorf("replace mode failed: %+v", p)
	}

	// 3B: Skip mode (stok lama dipertahankan)
	pRepo.products["BRG-EXIST"] = domain.Product{
		Code: "BRG-EXIST", Name: "Barang Lama", Category: "ATK", Price: 1000, Stock: 50, IsActive: true,
	}
	dataSkip := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"BRG-EXIST", "Barang Terupdate", "Elektronik", 6000, 999},
	})

	_, err = svc.StartImport(context.Background(), "skip.xlsx", dataSkip, "skip")
	if err != nil {
		t.Fatalf("StartImport skip error: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	pSkip, _ := pRepo.FindByCode(context.Background(), "BRG-EXIST")
	if pSkip.Name != "Barang Terupdate" || pSkip.Stock != 50 || pSkip.Price != 6000 {
		t.Errorf("skip mode should keep existing stock 50: %+v", pSkip)
	}
}

// -------------------------------------------------------------------------
// TEST 4: Import job tercatat di import_jobs
// -------------------------------------------------------------------------
func TestReq4_ImportJobRecordedInImportJobs(t *testing.T) {
	pRepo := newMockProductRepo()
	hRepo := newMockHistoryRepo()
	jRepo := newMockJobRepo()
	svc := etl.NewService(pRepo, hRepo, jRepo)

	data := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"BRG-JOB", "Barang Job", "ATK", 1000, 10},
	})

	job, err := svc.StartImport(context.Background(), "my_import.xlsx", data, "replace")
	if err != nil {
		t.Fatalf("StartImport error: %v", err)
	}

	// Immediately after creation, job should exist in RUNNING state
	savedJob, err := jRepo.FindByID(context.Background(), job.ID)
	if err != nil || savedJob == nil {
		t.Fatalf("job not saved in import_jobs: %v", err)
	}
	if savedJob.FileName != "my_import.xlsx" {
		t.Errorf("expected file_name 'my_import.xlsx', got '%s'", savedJob.FileName)
	}

	time.Sleep(100 * time.Millisecond)
	if savedJob.Status != domain.ImportStatusDone {
		t.Errorf("job should transition to DONE, got '%s'", savedJob.Status)
	}
}

// -------------------------------------------------------------------------
// TEST 5: Excel tanpa sheet Barang menghasilkan error yang jelas
// -------------------------------------------------------------------------
func TestReq5_MissingProductSheetProducesClearError(t *testing.T) {
	pRepo := newMockProductRepo()
	hRepo := newMockHistoryRepo()
	jRepo := newMockJobRepo()
	svc := etl.NewService(pRepo, hRepo, jRepo)

	// Create Excel with only "Lainnya" sheet
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "LembarLain")
	buf, _ := f.WriteToBuffer()

	_, err := svc.StartImport(context.Background(), "nosheet.xlsx", buf.Bytes(), "replace")
	if err == nil {
		t.Fatal("expected error for missing Barang sheet, got nil")
	}
	if !errors.Is(err, etl.ErrMissingSheet) && !strings.Contains(err.Error(), "tidak ditemukan") {
		t.Errorf("expected ErrMissingSheet or clear message, got: %v", err)
	}
}

// -------------------------------------------------------------------------
// TEST 6: Header wajib yang hilang menghasilkan error yang jelas
// -------------------------------------------------------------------------
func TestReq6_MissingRequiredHeaderProducesClearError(t *testing.T) {
	// Missing "Stok" and "Harga"
	data := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang"},
		{"BRG-01", "Buku"},
	})

	wb, err := etl.OpenWorkbook(data)
	if err != nil {
		t.Fatal(err)
	}
	defer wb.Close()

	_, err = etl.ParseWorkbook(wb)
	if err == nil {
		t.Fatal("expected error for missing required columns, got nil")
	}
	if !strings.Contains(err.Error(), "kolom wajib") {
		t.Errorf("expected clear error mentioning required columns, got: %v", err)
	}
}

// -------------------------------------------------------------------------
// TEST 7: Baris tidak valid tidak menyebabkan seluruh baris valid gagal disimpan
// -------------------------------------------------------------------------
func TestReq7_InvalidRowsDoNotAbortValidRows(t *testing.T) {
	pRepo := newMockProductRepo()
	hRepo := newMockHistoryRepo()
	jRepo := newMockJobRepo()
	svc := etl.NewService(pRepo, hRepo, jRepo)

	data := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"VALID-1", "Barang Valid 1", "ATK", 1000, 10},
		{"BAD-ROW", "Barang Rusak", "ATK", 2000, "NON_NUMERIC_STOK"}, // Invalid
		{"VALID-2", "Barang Valid 2", "ATK", 3000, 30},
	})

	job, err := svc.StartImport(context.Background(), "mixed.xlsx", data, "replace")
	if err != nil {
		t.Fatalf("StartImport error: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	curJob, _ := jRepo.FindByID(context.Background(), job.ID)
	if curJob.Status != domain.ImportStatusDone {
		t.Fatalf("expected DONE, got %s", curJob.Status)
	}
	if curJob.Inserted != 2 {
		t.Errorf("expected 2 valid products inserted, got %d", curJob.Inserted)
	}
	if curJob.Failed != 1 {
		t.Errorf("expected 1 failed row, got %d", curJob.Failed)
	}
	if len(curJob.Errors) != 1 {
		t.Errorf("expected 1 error recorded, got %d", len(curJob.Errors))
	}

	// Verify both valid products are saved in products collection
	if _, err := pRepo.FindByCode(context.Background(), "VALID-1"); err != nil {
		t.Error("VALID-1 should be in products collection")
	}
	if _, err := pRepo.FindByCode(context.Background(), "VALID-2"); err != nil {
		t.Error("VALID-2 should be in products collection")
	}
}

// -------------------------------------------------------------------------
// TEST 8: Database error mengubah status job menjadi failed
// -------------------------------------------------------------------------
func TestReq8_DatabaseErrorMarksJobFailed(t *testing.T) {
	pRepo := newMockProductRepo()
	pRepo.upsertErr = errors.New("mongodb connection lost") // Simulate DB error
	hRepo := newMockHistoryRepo()
	jRepo := newMockJobRepo()
	svc := etl.NewService(pRepo, hRepo, jRepo)

	data := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"FAIL-01", "Barang Gagal", "ATK", 1000, 10},
	})

	job, err := svc.StartImport(context.Background(), "dberr.xlsx", data, "replace")
	if err != nil {
		t.Fatalf("StartImport error: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	curJob, _ := jRepo.FindByID(context.Background(), job.ID)
	if curJob.Status != domain.ImportStatusFailed {
		t.Fatalf("expected FAILED, got %s", curJob.Status)
	}
	if len(curJob.Errors) == 0 || !strings.Contains(curJob.Errors[0].Reason, "mongodb connection lost") {
		t.Errorf("expected DB error message in job.Errors, got: %+v", curJob.Errors)
	}
}

// -------------------------------------------------------------------------
// TEST 9: Status done hanya diberikan setelah seluruh proses penyimpanan selesai
// -------------------------------------------------------------------------
func TestReq9_DoneStatusOnlyAfterStorageCompletes(t *testing.T) {
	pRepo := newMockProductRepo()
	hRepo := newMockHistoryRepo()
	jRepo := newMockJobRepo()

	storageCompleted := false
	pRepo.upsertHook = func(products []domain.Product) {
		time.Sleep(50 * time.Millisecond)
		storageCompleted = true
	}

	svc := etl.NewService(pRepo, hRepo, jRepo)

	data := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"TIMING-01", "Barang Timing", "ATK", 1000, 10},
	})

	job, _ := svc.StartImport(context.Background(), "timing.xlsx", data, "replace")

	// Check immediately before storage completes: status must NOT be DONE yet
	time.Sleep(10 * time.Millisecond)
	curJob, _ := jRepo.FindByID(context.Background(), job.ID)
	if curJob.Status == domain.ImportStatusDone {
		t.Fatal("job was marked DONE before storage completed!")
	}

	// Wait for storage and job completion
	time.Sleep(150 * time.Millisecond)
	curJob, _ = jRepo.FindByID(context.Background(), job.ID)
	if curJob.Status != domain.ImportStatusDone {
		t.Fatalf("expected DONE after storage, got %s", curJob.Status)
	}
	if !storageCompleted {
		t.Fatal("storage should be completed when job is DONE")
	}
}

// -------------------------------------------------------------------------
// TEST 10: Produk hasil import dapat ditemukan melalui repository / FindAll
// -------------------------------------------------------------------------
func TestReq10_ImportedProductsFindableInRepository(t *testing.T) {
	pRepo := newMockProductRepo()
	hRepo := newMockHistoryRepo()
	jRepo := newMockJobRepo()
	svc := etl.NewService(pRepo, hRepo, jRepo)

	data := makeTestExcel(t, "Barang", [][]any{
		{"Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"},
		{"SEARCH-001", "Buku Geografi", "Buku", 45000, 15},
		{"SEARCH-002", "Buku Fisika", "Buku", 50000, 20},
	})

	_, err := svc.StartImport(context.Background(), "findable.xlsx", data, "replace")
	if err != nil {
		t.Fatalf("StartImport error: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Verify FindAll finds the imported items
	prods, total, err := pRepo.FindAll(context.Background(), domain.ProductFilter{Query: "Geografi"})
	if err != nil {
		t.Fatalf("FindAll error: %v", err)
	}
	if total != 1 || len(prods) != 1 || prods[0].Code != "SEARCH-001" {
		t.Errorf("expected SEARCH-001 from FindAll, got %d results: %+v", total, prods)
	}

	// Verify FindByCode
	p, err := pRepo.FindByCode(context.Background(), "SEARCH-002")
	if err != nil || p.Name != "Buku Fisika" {
		t.Errorf("expected SEARCH-002 from FindByCode, got %+v, err: %v", p, err)
	}
}
