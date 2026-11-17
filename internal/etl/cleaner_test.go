package etl

import "testing"

func TestParsePrice(t *testing.T) {
	valid := []struct {
		in   string
		want int64
	}{
		{"Rp 12.500,00", 12500},
		{"12,500", 12500},
		{"12500.0", 12500},
		{"  3500 ", 3500},
		{"Rp 3.500", 3500},
		{"3,500.00", 3500},
		{"Rp. 1.250.000", 1250000},
		{"rp15000", 15000},
		{"0", 0},
		{"Rp\u00a07.500", 7500},
	}
	for _, tc := range valid {
		got, err := ParsePrice(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParsePrice(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}

	for _, in := range []string{"abc", "", "   ", "Rp", "-100", "12abc", "1.234,567,89"} {
		if got, err := ParsePrice(in); err == nil {
			t.Errorf("ParsePrice(%q) = %d; want error", in, got)
		}
	}
}

func TestParseStock(t *testing.T) {
	valid := []struct {
		in   string
		want int64
	}{
		{"120", 120},
		{"120 pcs", 120},
		{" 45 ", 45},
		{"1.200", 1200},
		{"12500.0", 12500},
		{"0", 0},
	}
	for _, tc := range valid {
		got, err := ParseStock(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseStock(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}

	for _, in := range []string{"N/A", "", "abc", "-5", "12,5", "12 5"} {
		if got, err := ParseStock(in); err == nil {
			t.Errorf("ParseStock(%q) = %d; want error", in, got)
		}
	}
}

func TestParseYear(t *testing.T) {
	if got, err := ParseYear("2026"); err != nil || got != 2026 {
		t.Errorf("ParseYear(2026) = %d, %v", got, err)
	}
	if got, err := ParseYear("2026.0"); err != nil || got != 2026 {
		t.Errorf("ParseYear(2026.0) = %d, %v", got, err)
	}
	for _, in := range []string{"1999", "2101", "abc", ""} {
		if got, err := ParseYear(in); err == nil {
			t.Errorf("ParseYear(%q) = %d; want error", in, got)
		}
	}
}

func TestParseMonth(t *testing.T) {
	valid := []struct {
		in   string
		want int
	}{
		{"5", 5},
		{"05", 5},
		{"12", 12},
		{"Mei", 5},
		{"May", 5},
		{"Desember", 12},
		{"dec", 12},
		{" Agu. ", 8},
	}
	for _, tc := range valid {
		got, err := ParseMonth(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseMonth(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}

	for _, in := range []string{"0", "13", "xyz", "", "-1"} {
		if got, err := ParseMonth(in); err == nil {
			t.Errorf("ParseMonth(%q) = %d; want error", in, got)
		}
	}
}

func TestParseQty(t *testing.T) {
	if got, err := ParseQty("87"); err != nil || got != 87 {
		t.Errorf("ParseQty(87) = %d, %v", got, err)
	}
	if got, err := ParseQty("-1"); err == nil {
		t.Errorf("ParseQty(-1) = %d; want error", got)
	}
}

func TestNormalizeCode(t *testing.T) {
	if got := NormalizeCode("  brg-0001 "); got != "BRG-0001" {
		t.Errorf("NormalizeCode = %q", got)
	}
}

func TestCleanText(t *testing.T) {
	if got := CleanText("  Buku   Tulis \t Sidu "); got != "Buku Tulis Sidu" {
		t.Errorf("CleanText = %q", got)
	}
}

func TestCleanCategory(t *testing.T) {
	if got := CleanCategory(""); got != "Lainnya" {
		t.Errorf("CleanCategory(empty) = %q", got)
	}
	if got := CleanCategory("  ATK "); got != "ATK" {
		t.Errorf("CleanCategory(ATK) = %q", got)
	}
}

func TestIsBlankRow(t *testing.T) {
	if !IsBlankRow(nil) || !IsBlankRow([]string{"", "  ", "\u00a0"}) {
		t.Error("expected blank row")
	}
	if IsBlankRow([]string{"", "x"}) {
		t.Error("expected non-blank row")
	}
}