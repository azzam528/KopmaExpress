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
	SheetDaily    = "Penjualan_Harian"

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
	fieldDate     = "date"
)

// Aliases are compared after normalizeKey, so "Kode Barang" and "kode_barang" both become "kodebarang".
var productAliases = map[string][]string{
	fieldCode:     {"kodebarang", "kode", "kodebrg", "code", "sku", "kodeproduk", "barcode"},
	fieldName:     {"namabarang", "nama", "namabrg", "barang", "name", "namaproduk", "produk", "item", "deskripsi", "description"},
	fieldCategory: {"kategori", "category", "jenis", "golongan", "kelompok"},
	fieldPrice:    {"harga", "hargasatuan", "hargajual", "price", "hargabarang", "unitprice"},
	fieldStock:    {"stok", "stock", "jumlahstok", "sisastok", "qty", "jumlah", "kuantitas", "totalstok"},
}

var historyAliases = map[string][]string{
	fieldCode:    {"kodebarang", "kode", "kodebrg", "code", "sku", "kodeproduk", "barcode"},
	fieldYear:    {"tahun", "year", "thn"},
	fieldMonth:   {"bulan", "month", "bln"},
	fieldQty:     {"qtyterjual", "qty", "jumlahterjual", "terjual", "kuantitas", "jumlah"},
	fieldRevenue: {"totalomzet", "omzet", "revenue", "totalpenjualan", "total"},
}

var dailyAliases = map[string][]string{
	fieldDate:    {"tanggal", "tgl", "date", "tanggaltransaksi", "tanggalpenjualan"},
	fieldCode:    {"kodebarang", "kode", "kodebrg", "code", "sku", "kodeproduk", "barcode"},
	fieldQty:     {"qtyterjual", "qty", "jumlahterjual", "terjual", "kuantitas", "jumlah"},
	fieldRevenue: {"totalomzet", "omzet", "revenue", "totalpenjualan", "total"},
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

// DailyParser membaca sheet Penjualan_Harian: satu baris = satu barang pada satu tanggal.
type DailyParser struct {
	cols    columnMap
	priceOf func(code string) (int64, bool)
}

// NewDailyParser menerima priceOf opsional yang dipakai saat omzet kosong (omzet = qty x harga).
func NewDailyParser(header RawRow, priceOf func(code string) (int64, bool)) (*DailyParser, error) {
	cols := mapColumns(header.Cells, dailyAliases)
	if err := requireColumns(header.Sheet, cols, dailyAliases, fieldDate, fieldCode, fieldQty); err != nil {
		return nil, err
	}
	return &DailyParser{cols: cols, priceOf: priceOf}, nil
}

func (p *DailyParser) Parse(row RawRow) (domain.DailySalesHistory, error) {
	code := NormalizeCode(p.cols.cell(row.Cells, fieldCode))
	if code == "" {
		return domain.DailySalesHistory{}, errors.New("kode barang kosong")
	}
	date, err := ParseDate(p.cols.cell(row.Cells, fieldDate))
	if err != nil {
		return domain.DailySalesHistory{}, err
	}
	qty, err := ParseQty(p.cols.cell(row.Cells, fieldQty))
	if err != nil {
		return domain.DailySalesHistory{}, err
	}

	var revenue int64
	if raw := p.cols.cell(row.Cells, fieldRevenue); strings.TrimSpace(raw) != "" {
		if revenue, err = ParseRevenue(raw); err != nil {
			return domain.DailySalesHistory{}, err
		}
	} else {
		price, ok := int64(0), false
		if p.priceOf != nil {
			price, ok = p.priceOf(code)
		}
		if !ok {
			return domain.DailySalesHistory{}, fmt.Errorf("omzet kosong dan harga barang '%s' tidak diketahui", code)
		}
		revenue = qty * price
	}

	return domain.DailySalesHistory{
		ProductCode: code,
		Date:        date,
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

var (
	ProductSheetNames = []string{"Barang", "Data Barang", "Daftar Barang", "Produk", "Master Barang", "Products", "Items"}
	HistorySheetNames = []string{"Riwayat_Penjualan", "Riwayat Penjualan", "Penjualan", "Riwayat", "Sales_History", "Sales History", "Sales"}
	DailySheetNames   = []string{"Penjualan_Harian", "Penjualan Harian", "Riwayat_Harian", "Riwayat Harian", "Harian", "Daily_Sales", "Daily Sales"}
)

// FindProductSheet locates the products sheet by checking common aliases.
// If not found and the workbook has only one sheet, it checks if that sheet has product headers.
func FindProductSheet(f *excelize.File) (string, bool) {
	for _, name := range ProductSheetNames {
		if s, ok := FindSheet(f, name); ok {
			return s, true
		}
	}
	// Fallback: If only 1 sheet exists in workbook, inspect its first non-blank row for product columns
	sheets := f.GetSheetList()
	if len(sheets) == 1 {
		firstSheet := sheets[0]
		var found bool
		_ = ReadSheet(f, firstSheet, func(r RawRow) error {
			if IsBlankRow(r.Cells) {
				return nil
			}
			cols := mapColumns(r.Cells, productAliases)
			if _, okCode := cols[fieldCode]; okCode {
				if _, okName := cols[fieldName]; okName {
					found = true
				}
			}
			return errors.New("done") // stop scanning after header
		})
		if found {
			return firstSheet, true
		}
	}
	return "", false
}

// FindHistorySheet locates the history sheet by checking common aliases.
func FindHistorySheet(f *excelize.File) (string, bool) {
	for _, name := range HistorySheetNames {
		if s, ok := FindSheet(f, name); ok {
			return s, true
		}
	}
	return "", false
}

// FindDailySheet locates the daily sales sheet by checking common aliases.
func FindDailySheet(f *excelize.File) (string, bool) {
	for _, name := range DailySheetNames {
		if s, ok := FindSheet(f, name); ok {
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
	Daily     []domain.DailySalesHistory
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

// ParseOptions tunes ParseWorkbookWith.
type ParseOptions struct {
	// KnownCode is asked about history rows whose code is not in the Barang sheet (typically a database lookup).
	// Nil skips the check.
	KnownCode func(code string) bool
}

// ParseWorkbook is the sequential reference implementation: one pass, no goroutines.
// The worker-pool pipeline should produce the same rows, and this doubles as the benchmark baseline.
func ParseWorkbook(f *excelize.File) (*ParseResult, error) {
	return ParseWorkbookWith(f, ParseOptions{})
}

func ParseWorkbookWith(f *excelize.File, opts ParseOptions) (*ParseResult, error) {
	res := &ParseResult{}

	productSheet, ok := FindProductSheet(f)
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

	prices := make(map[string]int64, len(res.Products))
	for _, p := range res.Products {
		prices[p.Code] = p.Price
	}
	priceOf := func(code string) (int64, bool) {
		v, ok := prices[code]
		return v, ok
	}
	// knownProduct memeriksa kode barang: ada di sheet Barang, atau (jika diminta) di database.
	knownProduct := func(code string) bool {
		if _, inFile := prices[code]; inFile {
			return true
		}
		return opts.KnownCode == nil || opts.KnownCode(code)
	}

	if historySheet, ok := FindHistorySheet(f); ok {
		err = walkSheet(f, historySheet, res, func(header RawRow) (func(RawRow) error, error) {
			parser, err := NewHistoryParser(header, priceOf)
			if err != nil {
				// Record header error as RowError for history sheet, but DO NOT abort the product import
				res.addError(NewRowError(header, err))
				return func(RawRow) error { return nil }, nil
			}
			return func(row RawRow) error {
				h, err := parser.Parse(row)
				if err != nil {
					return err
				}
				if !knownProduct(h.ProductCode) {
					return fmt.Errorf("kode barang '%s' tidak ada di sheet Barang maupun database", h.ProductCode)
				}
				res.History = append(res.History, h)
				return nil
			}, nil
		})
		if err != nil {
			return nil, err
		}
	}

	if dailySheet, ok := FindDailySheet(f); ok {
		err = walkSheet(f, dailySheet, res, func(header RawRow) (func(RawRow) error, error) {
			parser, err := NewDailyParser(header, priceOf)
			if err != nil {
				// Header harian salah tidak menggagalkan import barang/riwayat bulanan.
				res.addError(NewRowError(header, err))
				return func(RawRow) error { return nil }, nil
			}
			return func(row RawRow) error {
				d, err := parser.Parse(row)
				if err != nil {
					return err
				}
				if !knownProduct(d.ProductCode) {
					return fmt.Errorf("kode barang '%s' tidak ada di sheet Barang maupun database", d.ProductCode)
				}
				res.Daily = append(res.Daily, d)
				return nil
			}, nil
		})
		if err != nil {
			return nil, err
		}
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

// DedupeDaily keeps one record per (code, date); the last occurrence wins.
func DedupeDaily(in []domain.DailySalesHistory) []domain.DailySalesHistory {
	type key struct{ code, date string }
	idx := make(map[key]int, len(in))
	out := make([]domain.DailySalesHistory, 0, len(in))
	for _, d := range in {
		k := key{d.ProductCode, d.Date}
		if i, ok := idx[k]; ok {
			out[i] = d
			continue
		}
		idx[k] = len(out)
		out = append(out, d)
	}
	return out
}
