package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"smartcoop/internal/domain"
)

// stubProducts: repo palsu in-memory. Method yang tidak di-override akan panic
// (nil embed), sehingga test gagal keras bila service memanggil hal tak terduga.
type stubProducts struct {
	domain.ProductRepository

	byCode     map[string]*domain.Product
	createErr  error
	created    *domain.Product
	updated    *domain.Product
	deleted    string
	lastFilter domain.ProductFilter
}

func (f *stubProducts) FindAll(_ context.Context, flt domain.ProductFilter) ([]domain.Product, int64, error) {
	f.lastFilter = flt
	return []domain.Product{}, 0, nil
}

func (f *stubProducts) FindByCode(_ context.Context, code string) (*domain.Product, error) {
	p, ok := f.byCode[code]
	if !ok {
		return nil, domain.NewNotFoundError("Produk tidak ditemukan")
	}
	cp := *p
	return &cp, nil
}

func (f *stubProducts) Create(_ context.Context, p *domain.Product) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = p
	return nil
}

func (f *stubProducts) Update(_ context.Context, _ string, p *domain.Product) error {
	f.updated = p
	return nil
}

func (f *stubProducts) SoftDelete(_ context.Context, code string) error {
	f.deleted = code
	return nil
}

func newProductSvc(items ...*domain.Product) (domain.ProductService, *stubProducts) {
	stub := &stubProducts{byCode: map[string]*domain.Product{}}
	for _, p := range items {
		stub.byCode[p.Code] = p
	}
	return NewProductService(stub), stub
}

func detailFields(t *testing.T, ae *domain.AppError) map[string]bool {
	t.Helper()
	details, ok := ae.Details.([]domain.ValidationErrorDetail)
	if !ok {
		t.Fatalf("details bukan []ValidationErrorDetail: %T", ae.Details)
	}
	m := map[string]bool{}
	for _, d := range details {
		m[d.Field] = true
	}
	return m
}

func i64(v int64) *int64 { return &v }

func TestProductSvc_Create_NormalizesInput(t *testing.T) {
	svc, stub := newProductSvc()

	_, err := svc.Create(context.Background(), &domain.CreateProductRequest{
		Code: "  brg-0001 ", Name: "  Buku Tulis  ", Category: " ATK ", Price: 3500, Stock: 120,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.created == nil {
		t.Fatal("repo.Create tidak dipanggil")
	}
	if stub.created.Code != "BRG-0001" || stub.created.Name != "Buku Tulis" || stub.created.Category != "ATK" {
		t.Fatalf("input tidak dinormalisasi: %+v", stub.created)
	}
}

func TestProductSvc_Create_ValidationErrorsPerField(t *testing.T) {
	svc, stub := newProductSvc()

	_, err := svc.Create(context.Background(), &domain.CreateProductRequest{Price: -1, Stock: -5})
	ae := assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)

	fields := detailFields(t, ae)
	for _, want := range []string{"kode_barang", "nama_barang", "kategori", "harga", "stok"} {
		if !fields[want] {
			t.Errorf("harus ada error untuk field %q, got %v", want, fields)
		}
	}
	if stub.created != nil {
		t.Fatal("repo.Create tidak boleh dipanggil saat validasi gagal")
	}
}

func TestProductSvc_Create_CodeWithSpaceRejected(t *testing.T) {
	svc, _ := newProductSvc()

	_, err := svc.Create(context.Background(), &domain.CreateProductRequest{
		Code: "BRG 0001", Name: "X", Category: "ATK",
	})
	ae := assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
	if !detailFields(t, ae)["kode_barang"] {
		t.Fatal("kode berisi spasi harus ditolak di field kode_barang")
	}
}

func TestProductSvc_Create_NilRequest(t *testing.T) {
	svc, _ := newProductSvc()
	_, err := svc.Create(context.Background(), nil)
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
}

func TestProductSvc_Create_DuplicateBecomes409(t *testing.T) {
	req := &domain.CreateProductRequest{Code: "BRG-1", Name: "X", Category: "ATK"}

	t.Run("dari repo sebagai AppError", func(t *testing.T) {
		svc, stub := newProductSvc()
		stub.createErr = domain.NewDuplicateError("Kode barang sudah digunakan")
		_, err := svc.Create(context.Background(), req)
		assertAppError(t, err, domain.CodeDuplicate, http.StatusConflict)
	})

	t.Run("dari error mentah mongo E11000", func(t *testing.T) {
		svc, stub := newProductSvc()
		stub.createErr = errors.New("write exception: E11000 duplicate key error")
		_, err := svc.Create(context.Background(), req)
		assertAppError(t, err, domain.CodeDuplicate, http.StatusConflict)
	})
}

func TestProductSvc_GetByCode_InactiveIsNotFound(t *testing.T) {
	svc, _ := newProductSvc(
		&domain.Product{Code: "AKTIF", Name: "A", IsActive: true},
		&domain.Product{Code: "MATI", Name: "B", IsActive: false},
	)

	if _, err := svc.GetByCode(context.Background(), " aktif "); err != nil {
		t.Fatalf("barang aktif harus ditemukan (kode dinormalisasi): %v", err)
	}
	_, err := svc.GetByCode(context.Background(), "MATI")
	assertAppError(t, err, domain.CodeNotFound, http.StatusNotFound)
}

func TestProductSvc_Update_PartialKeepsOtherFields(t *testing.T) {
	svc, stub := newProductSvc(&domain.Product{
		Code: "BRG-1", Name: "Lama", Category: "ATK", Price: 1000, Stock: 5, IsActive: true,
	})

	got, err := svc.Update(context.Background(), "brg-1", &domain.UpdateProductRequest{Price: i64(2000)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Lama" || got.Category != "ATK" || got.Stock != 5 || got.Price != 2000 {
		t.Fatalf("hasil merge salah: %+v", got)
	}
	if stub.updated == nil || stub.updated.Price != 2000 {
		t.Fatal("repo.Update harus menerima data hasil merge")
	}
}

func TestProductSvc_Update_StockCanBeSetToZero(t *testing.T) {
	svc, _ := newProductSvc(&domain.Product{Code: "BRG-1", Name: "A", Category: "ATK", Stock: 5, IsActive: true})

	got, err := svc.Update(context.Background(), "BRG-1", &domain.UpdateProductRequest{Stock: i64(0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Stock != 0 {
		t.Fatalf("stok 0 harus diterima (pointer, bukan nil), got %d", got.Stock)
	}
}

func TestProductSvc_Update_RejectsNegativeAndInactive(t *testing.T) {
	svc, _ := newProductSvc(
		&domain.Product{Code: "AKTIF", Name: "A", Category: "ATK", IsActive: true},
		&domain.Product{Code: "MATI", Name: "B", Category: "ATK", IsActive: false},
	)

	_, err := svc.Update(context.Background(), "AKTIF", &domain.UpdateProductRequest{Price: i64(-1)})
	ae := assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
	if !detailFields(t, ae)["harga"] {
		t.Fatal("harga negatif harus ditolak di field harga")
	}

	_, err = svc.Update(context.Background(), "MATI", &domain.UpdateProductRequest{Price: i64(5)})
	assertAppError(t, err, domain.CodeNotFound, http.StatusNotFound)
}

func TestProductSvc_Delete(t *testing.T) {
	svc, stub := newProductSvc()

	if err := svc.Delete(context.Background(), "  brg-9 "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.deleted != "BRG-9" {
		t.Fatalf("kode harus dinormalisasi sebelum ke repo, got %q", stub.deleted)
	}

	err := svc.Delete(context.Background(), "   ")
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
}

func TestProductSvc_List_ForcesActiveAndClampsPaging(t *testing.T) {
	svc, stub := newProductSvc()

	inactive := false
	_, _, err := svc.List(context.Background(), domain.ProductFilter{
		Query: "  buku ", Page: -3, Limit: 9999, IsActive: &inactive,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := stub.lastFilter
	if f.IsActive == nil || !*f.IsActive {
		t.Fatal("List harus selalu memaksa is_active=true")
	}
	if f.Query != "buku" || f.Page != 1 || f.Limit != maxLimit {
		t.Fatalf("filter tidak dinormalisasi: %+v", f)
	}

	_, _, err = svc.List(context.Background(), domain.ProductFilter{Limit: 0})
	if err != nil || stub.lastFilter.Limit != defaultLimit {
		t.Fatalf("limit 0 harus jadi default %d, got %d (err=%v)", defaultLimit, stub.lastFilter.Limit, err)
	}
}

func TestProductSvc_List_UnknownSortRejected(t *testing.T) {
	svc, _ := newProductSvc()
	_, _, err := svc.List(context.Background(), domain.ProductFilter{Sort: "'; drop"})
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
}