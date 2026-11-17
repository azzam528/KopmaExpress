package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"smartcoop/internal/domain"
)

const (
	maxProductCodeLen = 30
	maxProductNameLen = 150
	maxProductCatLen  = 60

	// Batas atas wajar: mencegah overflow pada valuasi stok x harga di dashboard.
	maxProductPrice = 1_000_000_000
	maxProductStock = 1_000_000_000
)

// Nilai sort yang dikenali repo. "" dan "name" sama-sama berarti nama A-Z.
var allowedProductSorts = map[string]bool{
	"": true, "name": true,
	"stock_desc": true, "stock_asc": true,
	"price_asc": true, "price_desc": true,
	"newest": true,
}

type productService struct {
	repo domain.ProductRepository
}

var _ domain.ProductService = (*productService)(nil)

// NewProductService membuat service Master Barang (US-A1).
func NewProductService(repo domain.ProductRepository) domain.ProductService {
	return &productService{repo: repo}
}

// ------------------------------------------------------------------ List

// List selalu hanya mengembalikan barang AKTIF (barang yang di-soft-delete
// disembunyikan), apa pun yang dikirim klien.
func (s *productService) List(ctx context.Context, f domain.ProductFilter) ([]domain.Product, int64, error) {
	f.Query = strings.TrimSpace(f.Query)
	f.Category = strings.TrimSpace(f.Category)
	f.Sort = strings.TrimSpace(f.Sort)

	if !allowedProductSorts[f.Sort] {
		return nil, 0, domain.NewValidationError(
			"Parameter tidak valid",
			domain.ValidationErrorDetail{Field: "sort", Message: "nilai sort tidak dikenali"},
		)
	}

	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = defaultLimit
	}
	if f.Limit > maxLimit {
		f.Limit = maxLimit
	}

	active := true
	f.IsActive = &active

	return s.repo.FindAll(ctx, f)
}

// ------------------------------------------------------------------- Get

// GetByCode memperlakukan barang nonaktif sebagai tidak ditemukan (404).
func (s *productService) GetByCode(ctx context.Context, code string) (*domain.Product, error) {
	code = normalizeProductCode(code)
	if code == "" {
		return nil, domain.NewValidationError(
			"Kode barang wajib diisi",
			domain.ValidationErrorDetail{Field: "kode_barang", Message: "wajib diisi"},
		)
	}

	p, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if p == nil || !p.IsActive {
		return nil, domain.NewNotFoundError("Produk tidak ditemukan")
	}
	return p, nil
}

// ---------------------------------------------------------------- Create

func (s *productService) Create(ctx context.Context, req *domain.CreateProductRequest) (*domain.Product, error) {
	if req == nil {
		return nil, domain.NewValidationError("Body request wajib diisi")
	}

	p := &domain.Product{
		Code:     normalizeProductCode(req.Code),
		Name:     strings.TrimSpace(req.Name),
		Category: strings.TrimSpace(req.Category),
		Price:    req.Price,
		Stock:    req.Stock,
	}

	if details := validateProduct(p); len(details) > 0 {
		return nil, domain.NewValidationError("Data barang tidak valid", details...)
	}

	if err := s.repo.Create(ctx, p); err != nil {
		if isDuplicateKey(err) {
			return nil, domain.NewDuplicateError("Kode barang sudah digunakan")
		}
		return nil, err
	}
	return p, nil
}

// ---------------------------------------------------------------- Update

// Update bersifat parsial: nama/kategori kosong dan harga/stok nil berarti
// "tidak diubah". Kode barang tidak bisa diubah (diambil dari URL).
func (s *productService) Update(ctx context.Context, code string, req *domain.UpdateProductRequest) (*domain.Product, error) {
	if req == nil {
		return nil, domain.NewValidationError("Body request wajib diisi")
	}

	cur, err := s.GetByCode(ctx, code) // sekaligus menolak barang nonaktif
	if err != nil {
		return nil, err
	}

	merged := *cur
	if name := strings.TrimSpace(req.Name); name != "" {
		merged.Name = name
	}
	if cat := strings.TrimSpace(req.Category); cat != "" {
		merged.Category = cat
	}
	if req.Price != nil {
		merged.Price = *req.Price
	}
	if req.Stock != nil {
		merged.Stock = *req.Stock
	}

	if details := validateProduct(&merged); len(details) > 0 {
		return nil, domain.NewValidationError("Data barang tidak valid", details...)
	}

	if err := s.repo.Update(ctx, merged.Code, &merged); err != nil {
		return nil, err
	}
	return &merged, nil
}

// ---------------------------------------------------------------- Delete

// Delete = soft delete (is_active=false) agar riwayat order/penjualan tetap konsisten.
func (s *productService) Delete(ctx context.Context, code string) error {
	code = normalizeProductCode(code)
	if code == "" {
		return domain.NewValidationError(
			"Kode barang wajib diisi",
			domain.ValidationErrorDetail{Field: "kode_barang", Message: "wajib diisi"},
		)
	}
	return s.repo.SoftDelete(ctx, code)
}

// --------------------------------------------------------------- helpers

func normalizeProductCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// validateProduct mengembalikan daftar error per field (nama field = nama JSON,
// supaya frontend bisa menaruh pesan di bawah input yang tepat).
func validateProduct(p *domain.Product) []domain.ValidationErrorDetail {
	var d []domain.ValidationErrorDetail
	add := func(field, msg string) {
		d = append(d, domain.ValidationErrorDetail{Field: field, Message: msg})
	}

	switch {
	case p.Code == "":
		add("kode_barang", "wajib diisi")
	case utf8.RuneCountInString(p.Code) > maxProductCodeLen:
		add("kode_barang", "maksimal 30 karakter")
	case strings.ContainsAny(p.Code, " \t\r\n/?#%"):
		add("kode_barang", "tidak boleh mengandung spasi atau karakter / ? # %")
	}

	switch {
	case p.Name == "":
		add("nama_barang", "wajib diisi")
	case utf8.RuneCountInString(p.Name) > maxProductNameLen:
		add("nama_barang", "maksimal 150 karakter")
	}

	switch {
	case p.Category == "":
		add("kategori", "wajib diisi")
	case utf8.RuneCountInString(p.Category) > maxProductCatLen:
		add("kategori", "maksimal 60 karakter")
	}

	switch {
	case p.Price < 0:
		add("harga", "harus ≥ 0")
	case p.Price > maxProductPrice:
		add("harga", "terlalu besar")
	}

	switch {
	case p.Stock < 0:
		add("stok", "harus ≥ 0")
	case p.Stock > maxProductStock:
		add("stok", "terlalu besar")
	}

	return d
}