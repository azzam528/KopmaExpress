package excelgen

import (
	"reflect"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)

func rowsOf(t *testing.T, opts Options, sheet string) [][]string {
	t.Helper()
	f, err := Generate(opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows(%s): %v", sheet, err)
	}
	return rows
}

func TestGenerateCleanRowCounts(t *testing.T) {
	opts := Options{Items: 50, Months: 6, Members: 10, Seed: 1, Now: fixedNow}

	cases := map[string]int{
		SheetProducts: 51,
		SheetMembers:  11,
		SheetHistory:  301,
	}
	for sheet, want := range cases {
		if got := len(rowsOf(t, opts, sheet)); got != want {
			t.Errorf("%s: %d baris, want %d", sheet, got, want)
		}
	}
}

func TestGenerateSameSeedSameOutput(t *testing.T) {
	opts := Options{Items: 100, Months: 6, Dirty: true, Seed: 7, Now: fixedNow}
	a := rowsOf(t, opts, SheetHistory)
	b := rowsOf(t, opts, SheetHistory)
	if !reflect.DeepEqual(a, b) {
		t.Error("seed sama harus menghasilkan data sama")
	}
}

func TestGenerateDirtyAddsExtraRows(t *testing.T) {
	opts := Options{Items: 1000, Months: 6, Dirty: true, Seed: 1, Now: fixedNow}
	rows := rowsOf(t, opts, SheetProducts)

	if len(rows) <= 1001 {
		t.Errorf("mode dirty harus menambah baris kosong/duplikat, dapat %d baris", len(rows))
	}
}

func TestGenerateRejectsTooManyItems(t *testing.T) {
	if _, err := Generate(Options{Items: 100001}); err == nil {
		t.Error("Items di atas batas harus ditolak")
	}
}

func TestFormatRupiah(t *testing.T) {
	cases := map[int64]string{500: "500", 3500: "3.500", 12500: "12.500", 1250000: "1.250.000"}
	for in, want := range cases {
		if got := formatRupiah(in); got != want {
			t.Errorf("formatRupiah(%d) = %q, want %q", in, got, want)
		}
	}
}