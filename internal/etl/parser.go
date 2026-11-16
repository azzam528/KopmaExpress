package etl

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"smartcoop/internal/domain"
)

const (
	SheetProducts = "Barang"
	SheetHistory  = "Riwayat_Penjualan"

	// MaxRowErrors caps the error list kept per import (PRD §2.2); Failed still counts every row.
	MaxRowErrors = 1000

	sourceExcel = "excel"
)

const (
	fieldCode     = "code"
	fieldName     = "name"
	fieldCategory = "category"
	fieldPrice    = "price"
	fieldStock    = "stock"
	fieldYear     = "year"
	fieldMonth    = "month"
	fieldQty      = "qty"
	fieldRevenue  = "revenue"
)

// Aliases are compared after normalizeKey, so "Kode Barang" and "kode_barang" both become "kodebarang".
var productAliases = map[string][]string{
	fieldCode:     {"kodebarang", "kode", "kodebrg", "code", "sku"},
	fieldName:     {"namabarang", "nama", "namabrg", "barang", "name"},
	fieldCategory: {"kategori", "category", "jenis"},
	fieldPrice:    {"harga", "hargasatuan", "hargajual", "price"},
	fieldStock:    {"stok", "stock", "jumlahstok", "sisastok"},
}

var historyAliases = map[string][]string{
	fieldCode:    {"kodebarang", "kode", "kodebrg", "code", "sku"},
	fieldYear:    {"tahun", "year"},
	fieldMonth:   {"bulan", "month"},
	fieldQty:     {"qtyterjual", "qty", "jumlahterjual", "terjual"},
	fieldRevenue: {"totalomzet", "omzet", "revenue", "totalpenjualan"},
}

type RawRow struct {
	Sheet string
	Row   int
	Cells []string
}

func NewRowError(row RawRow, err error) domain.RowError {
	return domain.RowError{Sheet: row.Sheet, Row: row.Row, Reason: err.Error()}
}

type columnMap map[string]int

func (c columnMap) cell(cells []string, field string) string {
	i, ok := c[field]
	if !ok || i >= len(cells) {
		return ""
	}
	return cells[i]
}

func normalizeKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func mapColumns(header []string, aliases map[string][]string) columnMap {
	cols := columnMap{}
	for i, h := range header {
		key := normalizeKey(h)
		for field, names := range aliases {
			if _, taken := cols[field]; taken {
				continue
			}
			for _, n := range names {
				if n == key {
					cols[field] = i
					break
				}
			}
		}
	}
	return cols
}

func requireColumns(sheet string, cols columnMap, aliases map[string][]string, required ...string) error {
	var missing []string
	for _, field := range required {
		if _, ok := cols[field]; !ok {
			missing = append(missing, aliases[field][0])
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("sheet %s: kolom wajib tidak ditemukan: %s", sheet, strings.Join(missing, ", "))
	}
	return nil
}

type ProductParser struct {
	cols columnMap
}

func NewProductParser(header RawRow) (*ProductParser, error) {
	cols := mapColumns(header.Cells, productAliases)
	if err := requireColumns(header.Sheet, cols, productAliases, fieldCode, fieldName, fieldPrice, fieldStock); err != nil {
		return nil, err
	}
	return &ProductParser{cols: cols}, nil
}

func (p *ProductParser) Parse(row RawRow) (domain.Product, error) {
	code := NormalizeCode(p.cols.cell(row.Cells, fieldCode))
	if code == "" {
		return domain.Product{}, errors.New("kode barang kosong")
	}
	if utf8.RuneCountInString(code) > 30 {
		return domain.Product{}, fmt.Errorf("kode barang melebihi 30 karakter: '%s'", code)
	}

	name := CleanText(p.cols.cell(row.Cells, fieldName))
	if name == "" {
		return domain.Product{}, errors.New("nama barang kosong")
	}
	if utf8.RuneCountInString(name) > 150 {
		return domain.Product{}, errors.New("nama barang melebihi 150 karakter")
	}

	category := CleanCategory(p.cols.cell(row.Cells, fieldCategory))
	if utf8.RuneCountInString(category) > 60 {
		return domain.Product{}, errors.New("kategori melebihi 60 karakter")
	}

	price, err := ParsePrice(p.cols.cell(row.Cells, fieldPrice))
	if err != nil {
		return domain.Product{}, err
	}
	stock, err := ParseStock(p.cols.cell(row.Cells, fieldStock))
	if err != nil {
		return domain.Product{}, err
	}

	return domain.Product{
		Code:     code,
		Name:     name,
		Category: category,
		Price:    price,
		Stock:    stock,
		IsActive: true,
	}, nil
}

type HistoryParser struct {
	cols    columnMap
	priceOf func(code string) (int64, bool)
}

// NewHistoryParser takes an optional priceOf used when the revenue cell is blank (revenue = qty x price).
func NewHistoryParser(header RawRow, priceOf func(code string) (int64, bool)) (*HistoryParser, error) {
	cols := mapColumns(header.Cells, historyAliases)
	if err := requireColumns(header.Sheet, cols, historyAliases, fieldCode, fieldYear, fieldMonth, fieldQty); err != nil {
		return nil, err
	}
	return &HistoryParser{cols: cols, priceOf: priceOf}, nil
}

func (p *HistoryParser) Parse(row RawRow) (domain.MonthlySalesHistory, error) {
	code := NormalizeCode(p.cols.cell(row.Cells, fieldCode))
	if code == "" {
		return domain.MonthlySalesHistory{}, errors.New("kode barang kosong")
	}
	year, err := ParseYear(p.cols.cell(row.Cells, fieldYear))
	if err != nil {
		return domain.MonthlySalesHistory{}, err
	}
	month, err := ParseMonth(p.cols.cell(row.Cells, fieldMonth))
	if err != nil {
		return domain.MonthlySalesHistory{}, err
	}
	qty, err := ParseQty(p.cols.cell(row.Cells, fieldQty))
	if err != nil {
		return domain.MonthlySalesHistory{}, err
	}

	var revenue int64
	if raw := p.cols.cell(row.Cells, fieldRevenue); strings.TrimSpace(raw) != "" {
		if revenue, err = ParseRevenue(raw); err != nil {
			return domain.MonthlySalesHistory{}, err
		}
	} else {
		price, ok := int64(0), false
		if p.priceOf != nil {
			price, ok = p.priceOf(code)
		}
		if !ok {
			return domain.MonthlySalesHistory{}, fmt.Errorf("omzet kosong dan harga barang '%s' tidak diketahui", code)
		}
		revenue = qty * price
	}

	return domain.MonthlySalesHistory{
		ProductCode: code,
		Year:        year,
		Month:       month,
		QtySold:     qty,
		Revenue:     revenue,
		Source:      sourceExcel,
	}, nil
}

func OpenWorkbook(data []byte) (*excelize.File, error) {
	return excelize.OpenReader(bytes.NewReader(data))
}

// FindSheet matches a sheet name ignoring case, spaces and underscores.
func FindSheet(f *excelize.File, name string) (string, bool) {
	want := normalizeKey(name)
	for _, s := range f.GetSheetList() {
		if normalizeKey(s) == want {
			return s, true
		}
	}
	return "", false
}

// ReadSheet streams rows one by one without loading the whole sheet. Row is the 1-based Excel row number.
func ReadSheet(f *excelize.File, sheet string, emit func(RawRow) error) error {
	rows, err := f.Rows(sheet)
	if err != nil {
		return err
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		n++
		cells, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return fmt.Errorf("sheet %s baris %d: %w", sheet, n, err)
		}
		if err := emit(RawRow{Sheet: sheet, Row: n, Cells: cells}); err != nil {
			return err
		}
	}
	return rows.Error()
}

type ParseResult struct {
	Products  []domain.Product
	History   []domain.MonthlySalesHistory
	Errors    []domain.RowError
	Failed    int64
	BlankRows int64
}

func (r *ParseResult) addError(re domain.RowError) {
	r.Failed++
	if len(r.Errors) < MaxRowErrors {
		r.Errors = append(r.Errors, re)
	}
}

// ParseWorkbook is the sequential reference implementation: one pass, no goroutines.
// The worker-pool pipeline should produce the same rows, and this doubles as the benchmark baseline.
func ParseWorkbook(f *excelize.File) (*ParseResult, error) {
	res := &ParseResult{}

	productSheet, ok := FindSheet(f, SheetProducts)
	if !ok {
		return nil, fmt.Errorf("sheet '%s' tidak ditemukan", SheetProducts)
	}
	err := walkSheet(f, productSheet, res, func(header RawRow) (func(RawRow) error, error) {
		parser, err := NewProductParser(header)
		if err != nil {
			return nil, err
		}
		return func(row RawRow) error {
			p, err := parser.Parse(row)
			if err != nil {
				return err
			}
			res.Products = append(res.Products, p)
			return nil
		}, nil
	})
	if err != nil {
		return nil, err
	}

	historySheet, ok := FindSheet(f, SheetHistory)
	if !ok {
		return res, nil
	}
	prices := make(map[string]int64, len(res.Products))
	for _, p := range res.Products {
		prices[p.Code] = p.Price
	}
	priceOf := func(code string) (int64, bool) {
		v, ok := prices[code]
		return v, ok
	}
	err = walkSheet(f, historySheet, res, func(header RawRow) (func(RawRow) error, error) {
		parser, err := NewHistoryParser(header, priceOf)
		if err != nil {
			return nil, err
		}
		return func(row RawRow) error {
			h, err := parser.Parse(row)
			if err != nil {
				return err
			}
			res.History = append(res.History, h)
			return nil
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// walkSheet treats the first non-blank row as the header, then feeds each later row to the parse func
// that build returns. Row-level failures are recorded in res; only header problems abort.
func walkSheet(f *excelize.File, sheet string, res *ParseResult, build func(header RawRow) (func(RawRow) error, error)) error {
	var parse func(RawRow) error
	return ReadSheet(f, sheet, func(row RawRow) error {
		if IsBlankRow(row.Cells) {
			res.BlankRows++
			return nil
		}
		if parse == nil {
			p, err := build(row)
			if err != nil {
				return err
			}
			parse = p
			return nil
		}
		if err := parse(row); err != nil {
			res.addError(NewRowError(row, err))
		}
		return nil
	})
}

// DedupeProducts keeps one product per code; the last occurrence wins and first-seen order is kept.
// Needed because unordered BulkWrite upserts of the same code can race into duplicate-key errors.
func DedupeProducts(in []domain.Product) []domain.Product {
	idx := make(map[string]int, len(in))
	out := make([]domain.Product, 0, len(in))
	for _, p := range in {
		if i, ok := idx[p.Code]; ok {
			out[i] = p
			continue
		}
		idx[p.Code] = len(out)
		out = append(out, p)
	}
	return out
}

func DedupeHistory(in []domain.MonthlySalesHistory) []domain.MonthlySalesHistory {
	type key struct {
		code        string
		year, month int
	}
	idx := make(map[key]int, len(in))
	out := make([]domain.MonthlySalesHistory, 0, len(in))
	for _, h := range in {
		k := key{h.ProductCode, h.Year, h.Month}
		if i, ok := idx[k]; ok {
			out[i] = h
			continue
		}
		idx[k] = len(out)
		out = append(out, h)
	}
	return out
}