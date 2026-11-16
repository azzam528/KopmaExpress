package etl

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"smartcoop/internal/domain"
)

// buildWorkbook writes a small dirty workbook, saves it to bytes and reopens it,
// so the test exercises the same streaming reader that real uploads use.
func buildWorkbook(t *testing.T) *excelize.File {
	t.Helper()
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", SheetProducts); err != nil {
		t.Fatal(err)
	}
	if _, err := f.NewSheet(SheetHistory); err != nil {
		t.Fatal(err)
	}

	put := func(sheet string, row int, cells ...any) {
		t.Helper()
		cell, err := excelize.CoordinatesToCellName(1, row)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow(sheet, cell, &cells); err != nil {
			t.Fatal(err)
		}
	}

	put(SheetProducts, 1, "Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok")
	put(SheetProducts, 2, "brg-001", "  Buku   Tulis ", "ATK", "Rp 3.500", "120 pcs")
	// row 3 stays empty
	put(SheetProducts, 4, "BRG-002", "Pensil", "", 2500, 50)
	put(SheetProducts, 5, "BRG-003", "Spidol", "ATK", 4000, "N/A")
	put(SheetProducts, 6, "BRG-001", "Buku Tulis Baru", "ATK", 4000, 10)

	put(SheetHistory, 1, "Kode Barang", "Tahun", "Bulan", "Qty Terjual", "Total Omzet")
	put(SheetHistory, 2, "BRG-001", 2026, "Mei", 10, "")
	put(SheetHistory, 3, "BRG-002", 2026, 6, 20, 50000)
	put(SheetHistory, 4, "BRG-002", 1999, 7, 5, 1000)

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	wb, err := OpenWorkbook(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return wb
}

func findError(errs []domain.RowError, sheet string, row int) (domain.RowError, bool) {
	for _, e := range errs {
		if e.Sheet == sheet && e.Row == row {
			return e, true
		}
	}
	return domain.RowError{}, false
}

func TestParseWorkbook(t *testing.T) {
	res, err := ParseWorkbook(buildWorkbook(t))
	if err != nil {
		t.Fatalf("ParseWorkbook: %v", err)
	}

	if len(res.Products) != 3 {
		t.Fatalf("products = %d, want 3", len(res.Products))
	}
	first := res.Products[0]
	if first.Code != "BRG-001" || first.Name != "Buku Tulis" || first.Category != "ATK" || first.Price != 3500 || first.Stock != 120 || !first.IsActive {
		t.Errorf("product pertama salah: %+v", first)
	}
	if second := res.Products[1]; second.Category != "Lainnya" || second.Price != 2500 || second.Stock != 50 {
		t.Errorf("product kedua salah: %+v", second)
	}

	if len(res.History) != 2 {
		t.Fatalf("history = %d, want 2", len(res.History))
	}
	may := res.History[0]
	if may.ProductCode != "BRG-001" || may.Year != 2026 || may.Month != 5 || may.QtySold != 10 || may.Source != "excel" {
		t.Errorf("history pertama salah: %+v", may)
	}
	// Revenue was blank, so it falls back to qty x the last price seen for BRG-001.
	if may.Revenue != 40000 {
		t.Errorf("omzet default = %d, want 40000", may.Revenue)
	}
	if res.History[1].Revenue != 50000 {
		t.Errorf("omzet eksplisit = %d, want 50000", res.History[1].Revenue)
	}

	if res.BlankRows != 1 {
		t.Errorf("BlankRows = %d, want 1", res.BlankRows)
	}
	if res.Failed != 2 || len(res.Errors) != 2 {
		t.Fatalf("Failed = %d, errors = %d, want 2 dan 2", res.Failed, len(res.Errors))
	}
	if e, ok := findError(res.Errors, SheetProducts, 5); !ok || !strings.Contains(e.Reason, "stok") {
		t.Errorf("error stok baris 5 tidak ditemukan: %+v", res.Errors)
	}
	if e, ok := findError(res.Errors, SheetHistory, 4); !ok || !strings.Contains(e.Reason, "tahun") {
		t.Errorf("error tahun baris 4 tidak ditemukan: %+v", res.Errors)
	}
}

func TestParseWorkbookRequiresProductSheet(t *testing.T) {
	f := excelize.NewFile()
	if _, err := ParseWorkbook(f); err == nil {
		t.Error("workbook tanpa sheet Barang harus ditolak")
	}
}

func TestProductParserHeaderAliases(t *testing.T) {
	header := RawRow{Sheet: SheetProducts, Row: 1, Cells: []string{"KODE", "nama_barang", "Harga Jual", "Stock"}}
	p, err := NewProductParser(header)
	if err != nil {
		t.Fatalf("NewProductParser: %v", err)
	}
	got, err := p.Parse(RawRow{Sheet: SheetProducts, Row: 2, Cells: []string{"x-1", "Teh Botol", "Rp 5.000", "7"}})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Code != "X-1" || got.Price != 5000 || got.Stock != 7 || got.Category != "Lainnya" {
		t.Errorf("hasil salah: %+v", got)
	}
}

func TestProductParserMissingColumns(t *testing.T) {
	_, err := NewProductParser(RawRow{Sheet: SheetProducts, Row: 1, Cells: []string{"Kode Barang", "Nama Barang"}})
	if err == nil || !strings.Contains(err.Error(), "harga") || !strings.Contains(err.Error(), "stok") {
		t.Errorf("error harus menyebut kolom harga dan stok, dapat: %v", err)
	}
}

func TestProductParserRejectsBadRows(t *testing.T) {
	p, err := NewProductParser(RawRow{Sheet: SheetProducts, Row: 1, Cells: []string{"Kode Barang", "Nama Barang", "Harga", "Stok"}})
	if err != nil {
		t.Fatal(err)
	}
	bad := [][]string{
		{"", "Tanpa Kode", "1000", "1"},
		{"A1", "", "1000", "1"},
		{"A2", "Harga Salah", "abc", "1"},
		{"A3", "Stok Salah", "1000", "N/A"},
		{strings.Repeat("X", 31), "Kode Panjang", "1000", "1"},
	}
	for _, cells := range bad {
		if _, err := p.Parse(RawRow{Sheet: SheetProducts, Row: 2, Cells: cells}); err == nil {
			t.Errorf("baris %v harus ditolak", cells)
		}
	}
}

func TestHistoryParserWithoutPriceLookup(t *testing.T) {
	header := RawRow{Sheet: SheetHistory, Row: 1, Cells: []string{"Kode Barang", "Tahun", "Bulan", "Qty Terjual", "Total Omzet"}}
	p, err := NewHistoryParser(header, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Parse(RawRow{Cells: []string{"A1", "2026", "5", "10", ""}}); err == nil {
		t.Error("omzet kosong tanpa lookup harga harus ditolak")
	}
	got, err := p.Parse(RawRow{Cells: []string{"a1", "2026", "5", "10", "35000"}})
	if err != nil || got.ProductCode != "A1" || got.Revenue != 35000 {
		t.Errorf("hasil = %+v, err = %v", got, err)
	}
}

func TestFindSheetIgnoresCaseAndSeparators(t *testing.T) {
	f := excelize.NewFile()
	if _, err := f.NewSheet("riwayat penjualan"); err != nil {
		t.Fatal(err)
	}
	if name, ok := FindSheet(f, SheetHistory); !ok || name != "riwayat penjualan" {
		t.Errorf("FindSheet = %q, %v", name, ok)
	}
	if _, ok := FindSheet(f, "Tidak Ada"); ok {
		t.Error("sheet yang tidak ada harus ditolak")
	}
}

func TestDedupeProducts(t *testing.T) {
	in := []domain.Product{
		{Code: "A", Price: 1},
		{Code: "B", Price: 2},
		{Code: "A", Price: 3},
	}
	out := DedupeProducts(in)
	if len(out) != 2 || out[0].Code != "A" || out[0].Price != 3 || out[1].Code != "B" {
		t.Errorf("DedupeProducts = %+v", out)
	}
}

func TestDedupeHistory(t *testing.T) {
	in := []domain.MonthlySalesHistory{
		{ProductCode: "A", Year: 2026, Month: 5, QtySold: 1},
		{ProductCode: "A", Year: 2026, Month: 6, QtySold: 2},
		{ProductCode: "A", Year: 2026, Month: 5, QtySold: 9},
	}
	out := DedupeHistory(in)
	if len(out) != 2 || out[0].QtySold != 9 || out[1].Month != 6 {
		t.Errorf("DedupeHistory = %+v", out)
	}
}