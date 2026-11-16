package mongo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"smartcoop/internal/domain"
)

// Tes integrasi: butuh MongoDB replica set yang menyala (docker compose up -d mongo).
// URI diambil dari env MONGO_URI (default localhost). Tiap tes memakai database
// sementara bernama smartcoop_test_<timestamp> yang dihapus setelah selesai.
// Jika MongoDB tidak bisa dihubungi, tes dilewati (SKIP), bukan gagal.

// ---------------------------------------------------------------- helpers

func itSetup(t *testing.T) (*OrderRepo, *mongo.Database) {
	t.Helper()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017/?replicaSet=rs0"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := Connect(ctx, uri)
	if err != nil {
		t.Skipf("MongoDB tidak tersedia (%v); tes integrasi dilewati", err)
	}

	db := client.Database(fmt.Sprintf("smartcoop_test_%d", time.Now().UnixNano()))
	if err := EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}

	t.Cleanup(func() {
		c, cn := context.WithTimeout(context.Background(), 10*time.Second)
		defer cn()
		_ = db.Drop(c)
		_ = client.Disconnect(c)
	})

	return NewOrderRepo(client, db), db
}

func itSeedProduct(t *testing.T, db *mongo.Database, code string, price, stock int64) {
	t.Helper()
	now := time.Now()
	_, err := db.Collection("products").InsertOne(context.Background(), bson.M{
		"kode_barang": code, "name": "Produk " + code, "category": "TEST",
		"price": price, "stock": stock, "is_active": true,
		"created_at": now, "updated_at": now,
	})
	if err != nil {
		t.Fatalf("seed product %s: %v", code, err)
	}
}

func itStock(t *testing.T, db *mongo.Database, code string) int64 {
	t.Helper()
	var p struct {
		Stock int64 `bson:"stock"`
	}
	err := db.Collection("products").FindOne(context.Background(), bson.M{"kode_barang": code}).Decode(&p)
	if err != nil {
		t.Fatalf("read stock %s: %v", code, err)
	}
	return p.Stock
}

func itHistory(t *testing.T, db *mongo.Database, code string, year, month int) (qty, revenue int64, found bool) {
	t.Helper()
	var h struct {
		QtySold int64 `bson:"qty_sold"`
		Revenue int64 `bson:"revenue"`
	}
	err := db.Collection("monthly_sales_history").FindOne(context.Background(),
		bson.M{"product_code": code, "year": year, "month": month}).Decode(&h)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, 0, false
	}
	if err != nil {
		t.Fatalf("read history %s: %v", code, err)
	}
	return h.QtySold, h.Revenue, true
}

func itCountOrders(t *testing.T, db *mongo.Database) int64 {
	t.Helper()
	n, err := db.Collection("orders").CountDocuments(context.Background(), bson.M{})
	if err != nil {
		t.Fatalf("count orders: %v", err)
	}
	return n
}

func itItem(code string, qty, price int64) domain.OrderItem {
	return domain.OrderItem{
		ProductCode: code, ProductName: "Produk " + code,
		Qty: qty, UnitPrice: price, Subtotal: qty * price,
	}
}

func itOrder(orderNo, nim string, createdAt time.Time, items ...domain.OrderItem) *domain.Order {
	var total int64
	for _, it := range items {
		total += it.Subtotal
	}
	return &domain.Order{
		OrderNo: orderNo, MemberNIM: nim, MemberName: "Tester",
		Items: items, TotalAmount: total,
		Payment: domain.PayCash, Status: domain.OrderCompleted,
		CreatedAt: createdAt.Truncate(time.Millisecond),
	}
}

func itAppError(t *testing.T, err error, wantCode string) *domain.AppError {
	t.Helper()
	var ae *domain.AppError
	if !errors.As(err, &ae) || ae.Code != wantCode {
		t.Fatalf("want AppError %s, got %v", wantCode, err)
	}
	return ae
}

// ------------------------------------------------- 1. konkurensi (WAJIB)

// 200 goroutine memesan 1 unit dari barang bertok 100.
// Harus: tepat 100 sukses, 100 ditolak (409), stok akhir 0, tidak ada stok negatif.
func TestOrderRepo_Concurrent_NoOverselling(t *testing.T) {
	repo, db := itSetup(t)

	const stock, workers = 100, 200
	const price = 1000
	itSeedProduct(t, db, "BRG-RACE", price, stock)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var ok, insufficient, other int64
	var mu sync.Mutex
	var otherErrs []error

	now := time.Now()
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // lepas semua goroutine bersamaan
			o := itOrder(fmt.Sprintf("ORD-TEST-%06d", i), "2210101001", now, itItem("BRG-RACE", 1, price))
			err := repo.CreateWithTransaction(ctx, o)

			var ae *domain.AppError
			switch {
			case err == nil:
				atomic.AddInt64(&ok, 1)
			case errors.As(err, &ae) && ae.Code == domain.CodeInsufficientStock:
				atomic.AddInt64(&insufficient, 1)
			default:
				atomic.AddInt64(&other, 1)
				mu.Lock()
				otherErrs = append(otherErrs, err)
				mu.Unlock()
			}
		}(i)
	}

	began := time.Now()
	close(start)
	wg.Wait()
	t.Logf("%d request konkuren selesai dalam %v", workers, time.Since(began))

	if other != 0 {
		t.Fatalf("ada %d error tak terduga, contoh: %v", other, otherErrs[0])
	}
	if ok != stock {
		t.Errorf("sukses = %d, want %d", ok, stock)
	}
	if insufficient != workers-stock {
		t.Errorf("ditolak 409 = %d, want %d", insufficient, workers-stock)
	}
	if got := itStock(t, db, "BRG-RACE"); got != 0 {
		t.Errorf("stok akhir = %d, want 0", got)
	}
	if n := itCountOrders(t, db); n != stock {
		t.Errorf("jumlah order tersimpan = %d, want %d", n, stock)
	}

	year, month := periodOf(now)
	qty, rev, found := itHistory(t, db, "BRG-RACE", year, month)
	if !found || qty != stock || rev != stock*price {
		t.Errorf("history = qty %d, omzet %d (found=%v), want qty %d, omzet %d", qty, rev, found, stock, stock*price)
	}
}

// ------------------------------------------------ 2. atomisitas / rollback

// Barang A cukup, B dan C kurang -> seluruh pesanan ditolak, stok A tetap utuh,
// tidak ada order & tidak ada riwayat penjualan yang tersimpan.
func TestOrderRepo_Atomicity_RollbackOnShortage(t *testing.T) {
	repo, db := itSetup(t)
	itSeedProduct(t, db, "BRG-A", 3500, 10)
	itSeedProduct(t, db, "BRG-B", 5000, 1)
	itSeedProduct(t, db, "BRG-C", 2000, 0)

	now := time.Now()
	order := itOrder("ORD-TEST-ROLLBACK", "2210101001", now,
		itItem("BRG-A", 3, 3500),
		itItem("BRG-B", 5, 5000),
		itItem("BRG-C", 2, 2000),
	)

	err := repo.CreateWithTransaction(context.Background(), order)
	ae := itAppError(t, err, domain.CodeInsufficientStock)

	details, ok := ae.Details.([]domain.InsufficientStockDetail)
	if !ok || len(details) != 2 {
		t.Fatalf("details = %#v, want 2 barang kurang (B dan C)", ae.Details)
	}
	byCode := map[string]domain.InsufficientStockDetail{}
	for _, d := range details {
		byCode[d.ProductCode] = d
	}
	if d := byCode["BRG-B"]; d.Requested != 5 || d.Available != 1 {
		t.Errorf("detail B = %+v, want diminta 5 tersedia 1", d)
	}
	if d := byCode["BRG-C"]; d.Requested != 2 || d.Available != 0 {
		t.Errorf("detail C = %+v, want diminta 2 tersedia 0", d)
	}

	// rollback: tidak ada yang berubah
	if got := itStock(t, db, "BRG-A"); got != 10 {
		t.Errorf("stok A = %d, want 10 (harus rollback)", got)
	}
	if got := itStock(t, db, "BRG-B"); got != 1 {
		t.Errorf("stok B = %d, want 1", got)
	}
	if n := itCountOrders(t, db); n != 0 {
		t.Errorf("order tersimpan = %d, want 0", n)
	}
	year, month := periodOf(now)
	if _, _, found := itHistory(t, db, "BRG-A", year, month); found {
		t.Error("riwayat penjualan A tidak boleh tercatat saat pesanan ditolak")
	}
}

// ------------------------------------------------------------- 3. cancel

func TestOrderRepo_CreateThenCancel(t *testing.T) {
	repo, db := itSetup(t)
	ctx := context.Background()
	itSeedProduct(t, db, "BRG-A", 3500, 10)

	now := time.Now()
	order := itOrder("ORD-TEST-CANCEL", "2210101001", now, itItem("BRG-A", 3, 3500))
	if err := repo.CreateWithTransaction(ctx, order); err != nil {
		t.Fatalf("create: %v", err)
	}
	if order.ID.IsZero() {
		t.Error("order.ID harus terisi setelah create")
	}

	year, month := periodOf(now)

	// setelah create
	if got := itStock(t, db, "BRG-A"); got != 7 {
		t.Errorf("stok setelah create = %d, want 7", got)
	}
	qty, rev, found := itHistory(t, db, "BRG-A", year, month)
	if !found || qty != 3 || rev != 10500 {
		t.Errorf("history setelah create = %d/%d (found=%v), want 3/10500", qty, rev, found)
	}
	got, err := repo.FindByOrderNo(ctx, "ORD-TEST-CANCEL")
	if err != nil || got.Status != domain.OrderCompleted || got.TotalAmount != 10500 {
		t.Fatalf("FindByOrderNo: %+v, err=%v", got, err)
	}

	// cancel
	if err := repo.CancelWithTransaction(ctx, "ORD-TEST-CANCEL"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := itStock(t, db, "BRG-A"); got != 10 {
		t.Errorf("stok setelah cancel = %d, want 10", got)
	}
	qty, rev, _ = itHistory(t, db, "BRG-A", year, month)
	if qty != 0 || rev != 0 {
		t.Errorf("history setelah cancel = %d/%d, want 0/0", qty, rev)
	}
	got, _ = repo.FindByOrderNo(ctx, "ORD-TEST-CANCEL")
	if got == nil || got.Status != domain.OrderCancelled {
		t.Errorf("status setelah cancel = %+v, want CANCELLED", got)
	}

	// cancel kedua: ditolak dan stok TIDAK bertambah lagi
	err = repo.CancelWithTransaction(ctx, "ORD-TEST-CANCEL")
	itAppError(t, err, "ALREADY_CANCELLED")
	if got := itStock(t, db, "BRG-A"); got != 10 {
		t.Errorf("stok setelah cancel ganda = %d, want 10", got)
	}

	// order tidak ada
	err = repo.CancelWithTransaction(ctx, "ORD-TIDAK-ADA")
	itAppError(t, err, domain.CodeNotFound)

	_, err = repo.FindByOrderNo(ctx, "ORD-TIDAK-ADA")
	itAppError(t, err, domain.CodeNotFound)
}

// Dua pembatalan serentak: stok hanya boleh kembali sekali.
func TestOrderRepo_Concurrent_DoubleCancel(t *testing.T) {
	repo, db := itSetup(t)
	ctx := context.Background()
	itSeedProduct(t, db, "BRG-A", 1000, 10)

	order := itOrder("ORD-TEST-DBLCANCEL", "2210101001", time.Now(), itItem("BRG-A", 4, 1000))
	if err := repo.CreateWithTransaction(ctx, order); err != nil {
		t.Fatalf("create: %v", err)
	}

	var success int64
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repo.CancelWithTransaction(ctx, "ORD-TEST-DBLCANCEL"); err == nil {
				atomic.AddInt64(&success, 1)
			}
		}()
	}
	wg.Wait()

	if success != 1 {
		t.Errorf("pembatalan sukses = %d, want 1", success)
	}
	if got := itStock(t, db, "BRG-A"); got != 10 {
		t.Errorf("stok akhir = %d, want 10", got)
	}
}

// ----------------------------------------------------------- 4. FindAll

func TestOrderRepo_FindAll_Filters(t *testing.T) {
	repo, db := itSetup(t)
	ctx := context.Background()
	itSeedProduct(t, db, "BRG-A", 1000, 1000)

	now := time.Now()
	o1 := itOrder("ORD-TEST-F1", "NIM-A", now.Add(-3*time.Hour), itItem("BRG-A", 1, 1000))
	o2 := itOrder("ORD-TEST-F2", "NIM-B", now.Add(-2*time.Hour), itItem("BRG-A", 1, 1000))
	o3 := itOrder("ORD-TEST-F3", "NIM-A", now.Add(-1*time.Hour), itItem("BRG-A", 1, 1000))
	for _, o := range []*domain.Order{o1, o2, o3} {
		if err := repo.CreateWithTransaction(ctx, o); err != nil {
			t.Fatalf("create %s: %v", o.OrderNo, err)
		}
	}
	if err := repo.CancelWithTransaction(ctx, "ORD-TEST-F2"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	t.Run("filter nim, urut terbaru dulu", func(t *testing.T) {
		list, total, err := repo.FindAll(ctx, domain.OrderFilter{MemberNIM: "NIM-A"})
		if err != nil || total != 2 || len(list) != 2 {
			t.Fatalf("total=%d len=%d err=%v, want 2/2", total, len(list), err)
		}
		if list[0].OrderNo != "ORD-TEST-F3" || list[1].OrderNo != "ORD-TEST-F1" {
			t.Errorf("urutan = %s, %s; want F3 lalu F1", list[0].OrderNo, list[1].OrderNo)
		}
	})

	t.Run("paginasi", func(t *testing.T) {
		list, total, err := repo.FindAll(ctx, domain.OrderFilter{MemberNIM: "NIM-A", Page: 2, Limit: 1})
		if err != nil || total != 2 || len(list) != 1 || list[0].OrderNo != "ORD-TEST-F1" {
			t.Fatalf("halaman 2: total=%d list=%v err=%v", total, list, err)
		}
	})

	t.Run("filter status", func(t *testing.T) {
		list, total, err := repo.FindAll(ctx, domain.OrderFilter{Status: domain.OrderCancelled})
		if err != nil || total != 1 || len(list) != 1 || list[0].OrderNo != "ORD-TEST-F2" {
			t.Fatalf("CANCELLED: total=%d list=%v err=%v", total, list, err)
		}
	})

	t.Run("filter rentang tanggal", func(t *testing.T) {
		from := now.Add(-150 * time.Minute) // setelah o1 (-3h), sebelum o2 (-2h)
		list, total, err := repo.FindAll(ctx, domain.OrderFilter{From: &from})
		if err != nil || total != 2 || len(list) != 2 {
			t.Fatalf("from: total=%d len=%d err=%v, want 2/2", total, len(list), err)
		}
	})

	t.Run("hasil kosong berupa slice kosong, bukan nil", func(t *testing.T) {
		list, total, err := repo.FindAll(ctx, domain.OrderFilter{MemberNIM: "TIDAK-ADA"})
		if err != nil || total != 0 {
			t.Fatalf("total=%d err=%v", total, err)
		}
		if list == nil || len(list) != 0 {
			t.Errorf("list = %#v, want slice kosong non-nil", list)
		}
	})
}
