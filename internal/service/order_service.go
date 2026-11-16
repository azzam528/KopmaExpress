package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	"smartcoop/internal/domain"
)

const (
	maxOrderItems = 100
	maxItemQty    = 1_000_000
	defaultLimit  = 20
	maxLimit      = 100
	maxOrderNoTry = 3
)

// WIB: zona waktu untuk tanggal nomor pesanan.
var wib = time.FixedZone("WIB", 7*60*60)

// Service hanya butuh satu method dari repo produk dan anggota, jadi memakai
// interface sempit (bukan domain.ProductRepository/MemberRepository penuh).
type productFinder interface {
	FindByCode(ctx context.Context, code string) (*domain.Product, error)
}

type memberFinder interface {
	FindByNIM(ctx context.Context, nim string) (*domain.Member, error)
}

type orderService struct {
	orders   domain.OrderRepository
	products productFinder
	members  memberFinder
}

var _ domain.OrderService = (*orderService)(nil)

// Constructor order service.
func NewOrderService(
	orders domain.OrderRepository,
	products productFinder,
	members memberFinder,
) domain.OrderService {
	return &orderService{
		orders:   orders,
		products: products,
		members:  members,
	}
}

// ---------------------------------------------------------------- Create

// Create memvalidasi request, mengambil snapshot nama dan harga barang,
// menghitung total, membuat nomor pesanan, lalu menyimpan secara atomik.
// Pengecekan dan pengurangan stok dilakukan atomik oleh repository.
func (s *orderService) Create(
	ctx context.Context,
	req *domain.CreateOrderRequest,
) (*domain.Order, error) {
	if req == nil {
		return nil, domain.NewValidationError("Body request tidak boleh kosong")
	}

	if details := normalizeAndValidate(req); len(details) > 0 {
		return nil, domain.NewValidationError("Validasi gagal", details...)
	}

	// --- Anggota
	member, err := s.members.FindByNIM(ctx, req.MemberNIM)
	if err != nil {
		if isNotFound(err) {
			return nil, domain.NewNotFoundError(
				fmt.Sprintf("Anggota dengan NIM %s tidak ditemukan", req.MemberNIM),
			)
		}
		return nil, err
	}

	if member == nil {
		return nil, domain.NewNotFoundError(
			fmt.Sprintf("Anggota dengan NIM %s tidak ditemukan", req.MemberNIM),
		)
	}

	if !member.IsActive {
		return nil, domain.NewValidationError(
			"Anggota tidak aktif",
			domain.ValidationErrorDetail{
				Field:   "nim",
				Message: "anggota tidak aktif",
			},
		)
	}

	// --- Barang: snapshot nama dan harga saat transaksi
	items := make([]domain.OrderItem, 0, len(req.Items))
	var total int64
	var inactive []domain.ValidationErrorDetail

	for i, it := range req.Items {
		p, err := s.products.FindByCode(ctx, it.ProductCode)
		if err != nil {
			if isNotFound(err) {
				return nil, domain.NewNotFoundError(
					fmt.Sprintf("Barang %s tidak ditemukan", it.ProductCode),
				)
			}
			return nil, err
		}

		if p == nil {
			return nil, domain.NewNotFoundError(
				fmt.Sprintf("Barang %s tidak ditemukan", it.ProductCode),
			)
		}

		if !p.IsActive {
			inactive = append(inactive, domain.ValidationErrorDetail{
				Field:   fmt.Sprintf("items[%d].kode_barang", i),
				Message: fmt.Sprintf("barang %s tidak aktif", it.ProductCode),
			})
			continue
		}

		subtotal := p.Price * int64(it.Qty)

		items = append(items, domain.OrderItem{
			ProductCode: p.Code,
			ProductName: p.Name,
			Qty:         it.Qty,
			UnitPrice:   p.Price,
			Subtotal:    subtotal,
		})

		total += subtotal
	}

	if len(inactive) > 0 {
		return nil, domain.NewValidationError(
			"Ada barang yang tidak aktif",
			inactive...,
		)
	}

	// --- Bentuk order
	now := time.Now().Truncate(time.Millisecond)

	order := &domain.Order{
		MemberNIM:   member.NIM,
		MemberName:  member.Name,
		Items:       items,
		TotalAmount: total,
		Payment:     req.Payment,
		Status:      domain.OrderCompleted,
		CreatedAt:   now,
	}

	// --- Simpan secara atomik
	// Jika nomor pesanan duplikat, coba nomor baru.
	for attempt := 0; attempt < maxOrderNoTry; attempt++ {
		order.OrderNo = newOrderNo(now)

		err = s.orders.CreateWithTransaction(ctx, order)
		if err == nil {
			return order, nil
		}

		if !isDuplicateKey(err) {
			return nil, err
		}
	}

	return nil, err
}

// ------------------------------------------------------- List / Get / Cancel

func (s *orderService) List(
	ctx context.Context,
	f domain.OrderFilter,
) ([]domain.Order, int64, error) {
	f.MemberNIM = strings.TrimSpace(f.MemberNIM)
	f.Status = domain.OrderStatus(
		strings.ToUpper(strings.TrimSpace(string(f.Status))),
	)

	var details []domain.ValidationErrorDetail

	if f.Status != "" &&
		f.Status != domain.OrderCompleted &&
		f.Status != domain.OrderCancelled {
		details = append(details, domain.ValidationErrorDetail{
			Field:   "status",
			Message: "harus COMPLETED atau CANCELLED",
		})
	}

	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		details = append(details, domain.ValidationErrorDetail{
			Field:   "from",
			Message: "tidak boleh setelah 'to'",
		})
	}

	if len(details) > 0 {
		return nil, 0, domain.NewValidationError(
			"Parameter filter tidak valid",
			details...,
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

	return s.orders.FindAll(ctx, f)
}

func (s *orderService) GetByOrderNo(
	ctx context.Context,
	orderNo string,
) (*domain.Order, error) {
	orderNo = strings.TrimSpace(orderNo)

	if orderNo == "" {
		return nil, domain.NewValidationError("Nomor pesanan wajib diisi")
	}

	return s.orders.FindByOrderNo(ctx, orderNo)
}

func (s *orderService) Cancel(
	ctx context.Context,
	orderNo string,
) error {
	orderNo = strings.TrimSpace(orderNo)

	if orderNo == "" {
		return domain.NewValidationError("Nomor pesanan wajib diisi")
	}

	return s.orders.CancelWithTransaction(ctx, orderNo)
}

// --------------------------------------------------------------- Helpers

// normalizeAndValidate merapikan input dan mengumpulkan semua kesalahan validasi.
func normalizeAndValidate(
	req *domain.CreateOrderRequest,
) []domain.ValidationErrorDetail {
	var d []domain.ValidationErrorDetail

	req.MemberNIM = strings.TrimSpace(req.MemberNIM)

	if req.MemberNIM == "" {
		d = append(d, domain.ValidationErrorDetail{
			Field:   "nim",
			Message: "wajib diisi",
		})
	}

	req.Payment = domain.PaymentMethod(
		strings.ToUpper(strings.TrimSpace(string(req.Payment))),
	)

	switch req.Payment {
	case domain.PayCash,
		domain.PayTransfer,
		domain.PayQRIS,
		domain.PayKasbon:
	default:
		d = append(d, domain.ValidationErrorDetail{
			Field:   "metode_pembayaran",
			Message: "harus salah satu dari CASH, TRANSFER, QRIS, KASBON",
		})
	}

	if len(req.Items) == 0 {
		d = append(d, domain.ValidationErrorDetail{
			Field:   "items",
			Message: "minimal 1 barang",
		})
		return d
	}

	if len(req.Items) > maxOrderItems {
		d = append(d, domain.ValidationErrorDetail{
			Field:   "items",
			Message: fmt.Sprintf("maksimal %d barang per pesanan", maxOrderItems),
		})
		return d
	}

	seen := make(map[string]struct{}, len(req.Items))

	for i := range req.Items {
		it := &req.Items[i]
		it.ProductCode = strings.TrimSpace(it.ProductCode)

		if it.ProductCode == "" {
			d = append(d, domain.ValidationErrorDetail{
				Field:   fmt.Sprintf("items[%d].kode_barang", i),
				Message: "wajib diisi",
			})
		} else if _, dup := seen[it.ProductCode]; dup {
			d = append(d, domain.ValidationErrorDetail{
				Field:   fmt.Sprintf("items[%d].kode_barang", i),
				Message: "duplikat; gabungkan qty-nya dalam satu baris",
			})
		} else {
			seen[it.ProductCode] = struct{}{}
		}

		if it.Qty < 1 || it.Qty > maxItemQty {
			d = append(d, domain.ValidationErrorDetail{
				Field:   fmt.Sprintf("items[%d].qty", i),
				Message: fmt.Sprintf("harus antara 1 dan %d", maxItemQty),
			})
		}
	}

	return d
}

// newOrderNo menghasilkan ORD-YYYYMMDD-XXXXXX.
func newOrderNo(t time.Time) string {
	return fmt.Sprintf(
		"ORD-%s-%06d",
		t.In(wib).Format("20060102"),
		rand.Intn(1_000_000),
	)
}

func isNotFound(err error) bool {
	var appErr *domain.AppError

	if errors.As(err, &appErr) {
		return appErr.Code == domain.CodeNotFound
	}

	return errors.Is(err, mongo.ErrNoDocuments)
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}

	var we mongo.WriteException

	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}

	return strings.Contains(err.Error(), "E11000")
}
