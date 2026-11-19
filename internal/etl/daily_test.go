package etl

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseDate(t *testing.T) {
	valid := map[string]string{
		"2026-11-17":          "2026-11-17",
		"2026/11/17":          "2026-11-17",
		"17/11/2026":          "2026-11-17",
		"7/3/2026":            "2026-03-07",
		"17-11-2026":          "2026-11-17",
		"2026-11-17 00:00:00": "2026-11-17",
		"2026-11-17T08:30:00": "2026-11-17",
		"46343":               "2026-11-17", // nomor seri tanggal Excel
		"  2026-01-05  ":      "2026-01-05",
	}
	for in, want := range valid {
		got, err := ParseDate(in)
		if err != nil {
			t.Errorf("ParseDate(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseDate(%q) = %q, want %q", in, got, want)
		}
	}

	for _, in := range []string{"", "   ", "bukan tanggal", "2026-13-40", "31/02/2026", "1999-01-01"} {
		if got, err := ParseDate(in); err == nil {
			t.Errorf("ParseDate(%q) = %q, seharusnya error", in, got)
		}
	}
}

func buildDailyWorkbook(t *testing.T) *excelize.File {
	t.Helper()
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", SheetProducts); err != nil {
		t.Fatal(err)
	}
	if _, err := f.NewSheet(SheetDaily); err != nil {
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
	put(SheetProducts, 2, "BRG-001", "Buku Tulis", "ATK", 3500, 120)

	put(SheetDaily, 1, "Tanggal", "Kode Barang", "Qty Terjual", "Total Omzet")
	put(SheetDaily, 2, "2026-10-05", "brg-001", 10, 35000) // valid, kode dinormalkan
	put(SheetDaily, 3, "06/10/2026", "BRG-001", 4, "")     // valid, omzet dari harga: 4 x 3500
	put(SheetDaily, 4, "bukan tanggal", "BRG-001", 1, 3500) // error: tanggal
	put(SheetDaily, 5, "2026-10-07", "BRG-999", 1, "")     // error: omzet kosong dan harga barang tidak diketahui
	put(SheetDaily, 6, "2026-10-05", "BRG-001", 12, 42000) // duplikat (kode, tanggal) dengan baris 2

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

func TestParseWorkbookDailySheet(t *testing.T) {
	res, err := ParseWorkbook(buildDailyWorkbook(t))
	if err != nil {
		t.Fatalf("ParseWorkbook: %v", err)
	}

	if len(res.Daily) != 3 {
		t.Fatalf("daily = %d, want 3 (baris valid sebelum dedupe)", len(res.Daily))
	}
	if d := res.Daily[0]; d.ProductCode != "BRG-001" || d.Date != "2026-10-05" || d.QtySold != 10 || d.Revenue != 35000 {
		t.Errorf("baris harian pertama salah: %+v", d)
	}
	if d := res.Daily[1]; d.Date != "2026-10-06" || d.Revenue != 4*3500 {
		t.Errorf("omzet kosong harus qty x harga: %+v", d)
	}

	if res.Failed != 2 {
		t.Fatalf("failed = %d, want 2", res.Failed)
	}
	if _, ok := findError(res.Errors, SheetDaily, 4); !ok {
		t.Error("baris 4 (tanggal salah) harus tercatat sebagai error")
	}
	if _, ok := findError(res.Errors, SheetDaily, 5); !ok {
		t.Error("baris 5 (omzet kosong, harga tak diketahui) harus tercatat sebagai error")
	}

	deduped := DedupeDaily(res.Daily)
	if len(deduped) != 2 {
		t.Fatalf("setelah dedupe = %d, want 2", len(deduped))
	}
	if deduped[0].QtySold != 12 {
		t.Errorf("duplikat: baris terakhir harus menang, got qty %d", deduped[0].QtySold)
	}
}
