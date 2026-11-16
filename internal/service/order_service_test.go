package service

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"testing"
	"time"

	"smartcoop/internal/domain"
)

// ------------------------------------------------------------------ fakes

type fakeOrderRepo struct {
	created     []*domain.Order
	createCalls int
	dupFirstN   int // N panggilan pertama dibalas error duplicate key (E11000)
	createErr   error
	lastFilter  domain.OrderFilter
	cancelNo    string
}

func (f *fakeOrderRepo) CreateWithTransaction(_ context.Context, o *domain.Order) error {
	f.createCalls++
	if f.dupFirstN > 0 {
		f.dupFirstN--
		return errors.New("E11000 duplicate key error collection: orders index: idx_orders_order_no_unique")
	}
	if f.createErr != nil {
		return f.createErr
	}
	cp := *o
	f.created = append(f.created, &cp)
	return nil
}

func (f *fakeOrderRepo) FindAll(_ context.Context, flt domain.OrderFilter) ([]domain.Order, int64, error) {
	f.lastFilter = flt
	return []domain.Order{}, 0, nil
}

func (f *fakeOrderRepo) FindByOrderNo(_ context.Context, _ string) (*domain.Order, error) {
	return nil, domain.NewNotFoundError("Pesanan tidak ditemukan")
}

func (f *fakeOrderRepo) CancelWithTransaction(_ context.Context, orderNo string) error {
	f.cancelNo = orderNo
	return nil
}

// Embed interface: method yang tidak dipakai tes cukup dibiarkan nil.
type fakeProductRepo struct {
	domain.ProductRepository
	items map[string]*domain.Product
}

func (f *fakeProductRepo) FindByCode(_ context.Context, code string) (*domain.Product, error) {
	if p, ok := f.items[code]; ok {
		return p, nil
	}
	return nil, domain.NewNotFoundError("barang tidak ditemukan")
}

type fakeMemberRepo struct {
	domain.MemberRepository
	items map[string]*domain.Member
}

func (f *fakeMemberRepo) FindByNIM(_ context.Context, nim string) (*domain.Member, error) {
	if m, ok := f.items[nim]; ok {
		return m, nil
	}
	return nil, domain.NewNotFoundError("anggota tidak ditemukan")
}

// ---------------------------------------------------------------- helpers

func newTestService() (domain.OrderService, *fakeOrderRepo) {
	repo := &fakeOrderRepo{}
	products := &fakeProductRepo{items: map[string]*domain.Product{
		"BRG-0001": {Code: "BRG-0001", Name: "Buku Tulis Sidu", Price: 3500, Stock: 100, IsActive: true},
		"BRG-0007": {Code: "BRG-0007", Name: "Pulpen Pilot", Price: 5000, Stock: 50, IsActive: true},
		"BRG-OFF":  {Code: "BRG-OFF", Name: "Barang Nonaktif", Price: 1000, Stock: 10, IsActive: false},
	}}
	members := &fakeMemberRepo{items: map[string]*domain.Member{
		"2210101001": {NIM: "2210101001", Name: "Andi Pratama", IsActive: true},
		"2210101002": {NIM: "2210101002", Name: "Budi Nonaktif", IsActive: false},
	}}
	return NewOrderService(repo, products, members), repo
}

// validReq membuat request baru tiap dipanggil (service boleh memodifikasi input).
func validReq() *domain.CreateOrderRequest {
	return &domain.CreateOrderRequest{
		MemberNIM: "2210101001",
		Payment:   domain.PayQRIS,
		Items: []domain.OrderItemRequest{
			{ProductCode: "BRG-0001", Qty: 3},
			{ProductCode: "BRG-0007", Qty: 1},
		},
	}
}

func assertAppError(t *testing.T, err error, wantCode string, wantStatus int) *domain.AppError {
	t.Helper()
	var ae *domain.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("want AppError %s, got %v", wantCode, err)
	}
	if ae.Code != wantCode || ae.HTTPStatus != wantStatus {
		t.Fatalf("want %s/%d, got %s/%d (%s)", wantCode, wantStatus, ae.Code, ae.HTTPStatus, ae.Message)
	}
	return ae
}

// ------------------------------------------------------------------ tests

func TestCreate_Success_SnapshotAndTotal(t *testing.T) {
	svc, repo := newTestService()

	order, err := svc.Create(context.Background(), validReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if order.TotalAmount != 15500 { // 3*3500 + 1*5000
		t.Errorf("total = %d, want 15500", order.TotalAmount)
	}
	if order.Status != domain.OrderCompleted {
		t.Errorf("status = %s, want COMPLETED", order.Status)
	}
	if order.MemberName != "Andi Pratama" || order.MemberNIM != "2210101001" {
		t.Errorf("member snapshot salah: %s / %s", order.MemberNIM, order.MemberName)
	}
	if len(order.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(order.Items))
	}
	first := order.Items[0]
	if first.ProductName != "Buku Tulis Sidu" || first.UnitPrice != 3500 || first.Subtotal != 10500 || first.Qty != 3 {
		t.Errorf("snapshot item[0] salah: %+v", first)
	}
	if !regexp.MustCompile(`^ORD-\d{8}-\d{6}$`).MatchString(order.OrderNo) {
		t.Errorf("format order_no salah: %s", order.OrderNo)
	}
	if len(repo.created) != 1 || repo.createCalls != 1 {
		t.Errorf("repo.Create dipanggil %d kali, want 1", repo.createCalls)
	}
}

func TestCreate_PaymentIsCaseInsensitive(t *testing.T) {
	svc, _ := newTestService()
	req := validReq()
	req.Payment = "  qris "

	order, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Payment != domain.PayQRIS {
		t.Errorf("payment = %s, want QRIS", order.Payment)
	}
}

func TestCreate_ValidationErrors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(r *domain.CreateOrderRequest)
	}{
		{"nim kosong", func(r *domain.CreateOrderRequest) { r.MemberNIM = "  " }},
		{"metode bayar salah", func(r *domain.CreateOrderRequest) { r.Payment = "BITCOIN" }},
		{"metode bayar kosong", func(r *domain.CreateOrderRequest) { r.Payment = "" }},
		{"items kosong", func(r *domain.CreateOrderRequest) { r.Items = nil }},
		{"qty nol", func(r *domain.CreateOrderRequest) { r.Items[0].Qty = 0 }},
		{"qty negatif", func(r *domain.CreateOrderRequest) { r.Items[0].Qty = -2 }},
		{"kode barang kosong", func(r *domain.CreateOrderRequest) { r.Items[0].ProductCode = "" }},
		{"kode barang duplikat", func(r *domain.CreateOrderRequest) { r.Items[1].ProductCode = r.Items[0].ProductCode }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := newTestService()
			req := validReq()
			tc.mutate(req)

			_, err := svc.Create(context.Background(), req)
			assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
			if repo.createCalls != 0 {
				t.Errorf("repo tidak boleh dipanggil saat validasi gagal")
			}
		})
	}
}

func TestCreate_NilRequest(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.Create(context.Background(), nil)
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
}

func TestCreate_LookupErrors(t *testing.T) {
	t.Run("anggota tidak ditemukan -> 404", func(t *testing.T) {
		svc, repo := newTestService()
		req := validReq()
		req.MemberNIM = "0000000000"
		_, err := svc.Create(context.Background(), req)
		assertAppError(t, err, domain.CodeNotFound, http.StatusNotFound)
		if repo.createCalls != 0 {
			t.Error("repo tidak boleh dipanggil")
		}
	})

	t.Run("anggota nonaktif -> 422", func(t *testing.T) {
		svc, _ := newTestService()
		req := validReq()
		req.MemberNIM = "2210101002"
		_, err := svc.Create(context.Background(), req)
		assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
	})

	t.Run("barang tidak ditemukan -> 404", func(t *testing.T) {
		svc, _ := newTestService()
		req := validReq()
		req.Items[1].ProductCode = "BRG-XXXX"
		_, err := svc.Create(context.Background(), req)
		assertAppError(t, err, domain.CodeNotFound, http.StatusNotFound)
	})

	t.Run("barang nonaktif -> 422", func(t *testing.T) {
		svc, _ := newTestService()
		req := validReq()
		req.Items[0].ProductCode = "BRG-OFF"
		_, err := svc.Create(context.Background(), req)
		assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
	})
}

func TestCreate_InsufficientStockFromRepoIsPassedThrough(t *testing.T) {
	svc, repo := newTestService()
	repo.createErr = domain.NewInsufficientStockError("Stok tidak mencukupi",
		domain.InsufficientStockDetail{ProductCode: "BRG-0007", Requested: 5, Available: 2})

	_, err := svc.Create(context.Background(), validReq())
	ae := assertAppError(t, err, domain.CodeInsufficientStock, http.StatusConflict)

	details, ok := ae.Details.([]domain.InsufficientStockDetail)
	if !ok || len(details) != 1 || details[0].Available != 2 {
		t.Errorf("details stok kurang tidak diteruskan: %#v", ae.Details)
	}
}

func TestCreate_RetriesWhenOrderNoCollides(t *testing.T) {
	t.Run("berhasil di percobaan ke-3", func(t *testing.T) {
		svc, repo := newTestService()
		repo.dupFirstN = 2
		if _, err := svc.Create(context.Background(), validReq()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.createCalls != 3 {
			t.Errorf("calls = %d, want 3", repo.createCalls)
		}
	})

	t.Run("menyerah setelah 3 percobaan", func(t *testing.T) {
		svc, repo := newTestService()
		repo.dupFirstN = 10
		if _, err := svc.Create(context.Background(), validReq()); err == nil {
			t.Fatal("want error")
		}
		if repo.createCalls != maxOrderNoTry {
			t.Errorf("calls = %d, want %d", repo.createCalls, maxOrderNoTry)
		}
	})

	t.Run("error lain tidak di-retry", func(t *testing.T) {
		svc, repo := newTestService()
		repo.createErr = errors.New("koneksi putus")
		if _, err := svc.Create(context.Background(), validReq()); err == nil {
			t.Fatal("want error")
		}
		if repo.createCalls != 1 {
			t.Errorf("calls = %d, want 1", repo.createCalls)
		}
	})
}

func TestNewOrderNo_UsesWIBDate(t *testing.T) {
	// 16 Nov 2026 20:00 UTC = 17 Nov 2026 03:00 WIB
	got := newOrderNo(time.Date(2026, 11, 16, 20, 0, 0, 0, time.UTC))
	if want := "ORD-20261117-"; got[:len(want)] != want {
		t.Errorf("order_no = %s, want prefix %s", got, want)
	}
}

func TestList_Validation(t *testing.T) {
	t.Run("status tidak valid", func(t *testing.T) {
		svc, _ := newTestService()
		_, _, err := svc.List(context.Background(), domain.OrderFilter{Status: "DONE"})
		assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
	})

	t.Run("from setelah to", func(t *testing.T) {
		svc, _ := newTestService()
		to := time.Now().Add(-time.Hour)
		from := time.Now()
		_, _, err := svc.List(context.Background(), domain.OrderFilter{From: &from, To: &to})
		assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
	})
}

func TestList_NormalizesPagingAndFilter(t *testing.T) {
	svc, repo := newTestService()

	_, _, err := svc.List(context.Background(), domain.OrderFilter{
		MemberNIM: " 2210101001 ", Status: "completed", Page: 0, Limit: 9999,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := repo.lastFilter
	if f.Page != 1 || f.Limit != maxLimit {
		t.Errorf("paging = %d/%d, want 1/%d", f.Page, f.Limit, maxLimit)
	}
	if f.MemberNIM != "2210101001" || f.Status != domain.OrderCompleted {
		t.Errorf("filter tidak dinormalisasi: %+v", f)
	}

	_, _, _ = svc.List(context.Background(), domain.OrderFilter{})
	if repo.lastFilter.Limit != defaultLimit {
		t.Errorf("default limit = %d, want %d", repo.lastFilter.Limit, defaultLimit)
	}
}

func TestCancelAndGet_RequireOrderNo(t *testing.T) {
	svc, repo := newTestService()

	err := svc.Cancel(context.Background(), "   ")
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)

	_, err = svc.GetByOrderNo(context.Background(), "")
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)

	if err := svc.Cancel(context.Background(), " ORD-20261116-000123 "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.cancelNo != "ORD-20261116-000123" {
		t.Errorf("order_no tidak di-trim: %q", repo.cancelNo)
	}
}
