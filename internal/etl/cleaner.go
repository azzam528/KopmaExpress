package etl

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultCategory = "Lainnya"
	maxAmount       = 1e15
)

var (
	moneyRe   = regexp.MustCompile(`^-?[0-9][0-9.,]*$`)
	leadNumRe = regexp.MustCompile(`^-?[0-9][0-9.,]*`)
	rpPrefix  = regexp.MustCompile(`(?i)^\s*rp\.?`)
)

var monthNames = map[string]int{
	"januari": 1, "january": 1, "jan": 1,
	"februari": 2, "pebruari": 2, "february": 2, "feb": 2, "peb": 2,
	"maret": 3, "march": 3, "mar": 3,
	"april": 4, "apr": 4,
	"mei": 5, "may": 5,
	"juni": 6, "june": 6, "jun": 6,
	"juli": 7, "july": 7, "jul": 7,
	"agustus": 8, "august": 8, "agu": 8, "agt": 8, "aug": 8,
	"september": 9, "sep": 9, "sept": 9,
	"oktober": 10, "october": 10, "okt": 10, "oct": 10,
	"november": 11, "nov": 11, "nop": 11,
	"desember": 12, "december": 12, "des": 12, "dec": 12,
}

func NormalizeCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func CleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func CleanCategory(s string) string {
	if c := CleanText(s); c != "" {
		return c
	}
	return defaultCategory
}

// IsBlankRow reports whether every cell is empty or whitespace-only.
func IsBlankRow(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// ParsePrice converts "Rp 12.500,00", "12,500", "12500.0" and similar into rupiah.
func ParsePrice(raw string) (int64, error) {
	return parseMoney(raw, "harga")
}

func ParseRevenue(raw string) (int64, error) {
	return parseMoney(raw, "omzet")
}

// ParseStock accepts a trailing unit such as "120 pcs".
func ParseStock(raw string) (int64, error) {
	return parseWhole(raw, "stok")
}

func ParseQty(raw string) (int64, error) {
	return parseWhole(raw, "qty terjual")
}

func ParseYear(raw string) (int, error) {
	n, err := parseWhole(raw, "tahun")
	if err != nil {
		return 0, err
	}
	if n < 2000 || n > 2100 {
		return 0, fmt.Errorf("tahun di luar rentang 2000-2100: '%s'", raw)
	}
	return int(n), nil
}

// ParseMonth accepts 1-12 or an Indonesian/English month name.
func ParseMonth(raw string) (int, error) {
	if m, ok := monthNames[strings.ToLower(strings.Trim(raw, ". \t"))]; ok {
		return m, nil
	}
	n, err := parseWhole(raw, "bulan")
	if err != nil {
		return 0, err
	}
	if n < 1 || n > 12 {
		return 0, fmt.Errorf("bulan di luar rentang 1-12: '%s'", raw)
	}
	return int(n), nil
}

func parseMoney(raw, label string) (int64, error) {
	s := rpPrefix.ReplaceAllString(raw, "")
	// Fields also strips non-breaking spaces that Excel exports often contain.
	s = strings.Join(strings.Fields(s), "")
	if s == "" {
		return 0, fmt.Errorf("%s kosong", label)
	}
	if !moneyRe.MatchString(s) {
		return 0, fmt.Errorf("%s tidak valid: '%s'", label, raw)
	}
	f, err := parseNumber(s, label, raw)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f)), nil
}

func parseWhole(raw, label string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("%s kosong", label)
	}
	token := leadNumRe.FindString(s)
	if token == "" || strings.ContainsAny(s[len(token):], "0123456789") {
		return 0, fmt.Errorf("%s tidak valid: '%s'", label, raw)
	}
	f, err := parseNumber(token, label, raw)
	if err != nil {
		return 0, err
	}
	if f != math.Trunc(f) {
		return 0, fmt.Errorf("%s harus bilangan bulat: '%s'", label, raw)
	}
	return int64(f), nil
}

func parseNumber(s, label, raw string) (float64, error) {
	negative := strings.HasPrefix(s, "-")
	canon, err := canonicalNumber(strings.TrimPrefix(s, "-"))
	if err != nil {
		return 0, fmt.Errorf("%s tidak valid: '%s'", label, raw)
	}
	f, err := strconv.ParseFloat(canon, 64)
	if err != nil || f >= maxAmount {
		return 0, fmt.Errorf("%s tidak valid: '%s'", label, raw)
	}
	if negative && f != 0 {
		return 0, fmt.Errorf("%s tidak boleh negatif: '%s'", label, raw)
	}
	return f, nil
}

// canonicalNumber turns a digits-and-separators string into "123.45" form.
// A lone separator followed by exactly three digits is a thousands separator
// ("12,500" and "3.500" are both whole rupiah), anything else is decimal.
func canonicalNumber(s string) (string, error) {
	dots := strings.Count(s, ".")
	commas := strings.Count(s, ",")

	decimalSep := ""
	switch {
	case dots > 0 && commas > 0:
		decimalSep = ","
		if strings.LastIndex(s, ".") > strings.LastIndex(s, ",") {
			decimalSep = "."
		}
	case dots+commas == 1:
		sep := "."
		if commas == 1 {
			sep = ","
		}
		if len(s)-strings.Index(s, sep)-1 != 3 {
			decimalSep = sep
		}
	}
	if decimalSep != "" && strings.Count(s, decimalSep) != 1 {
		return "", errors.New("pemisah desimal ganda")
	}

	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case decimalSep != "" && string(r) == decimalSep:
			b.WriteByte('.')
		}
	}
	return b.String(), nil
}