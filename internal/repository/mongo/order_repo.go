package mongo

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

	"smartcoop/internal/domain"
)

// errInsufficientStock dipakai sebagai sinyal internal untuk meng-abort
// transaksi ketika ada barang yang stoknya kurang. Error ini TIDAK dikirim ke
// luar; diganti dengan domain.NewInsufficientStockError (HTTP 409).
var errInsufficientStock = errors.New("insufficient stock")

// wib dipakai untuk menentukan bulan/tahun di monthly_sales_history.
// Create dan Cancel HARUS memakai zona waktu yang sama supaya pesanan di
// pergantian bulan tidak tercatat di bulan yang berbeda.
var wib = time.FixedZone("WIB", 7*60*60)

var orderTxnOpts = options.Transaction().
	SetReadConcern(readconcern.Snapshot()).
	SetWriteConcern(writeconcern.Majority())

// OrderRepo mengimplementasikan domain.OrderRepository (PRD §6.4, §8.3).
type OrderRepo struct {
	client   *mongo.Client
	products *mongo.Collection
	history  *mongo.Collection
	orders   *mongo.Collection
}

var _ domain.OrderRepository = (*OrderRepo)(nil)

func NewOrderRepo(client *mongo.Client, db *mongo.Database) *OrderRepo {
	return &OrderRepo{
		client:   client,
		products: db.Collection("products"),
		history:  db.Collection("monthly_sales_history"),
		orders:   db.Collection("orders"),
	}
}

// periodOf mengembalikan (tahun, bulan 1-12) menurut zona WIB.
func periodOf(t time.Time) (int, int) {
	t = t.In(wib)
	return t.Year(), int(t.Month())
}

func (r *OrderRepo) withTx(ctx context.Context, fn func(sc mongo.SessionContext) (interface{}, error)) error {
	sess, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer sess.EndSession(ctx)

	_, err = sess.WithTransaction(ctx, fn, orderTxnOpts)
	return err
}

// availableStock membaca sisa stok untuk keperluan pesan error 409.
func (r *OrderRepo) availableStock(ctx context.Context, code string) int64 {
	var p struct {
		Stock    int64 `bson:"stock"`
		IsActive bool  `bson:"is_active"`
	}
	err := r.products.FindOne(ctx, bson.M{"kode_barang": code}).Decode(&p)
	if err != nil || !p.IsActive {
		return 0
	}
	return p.Stock
}

// CreateWithTransaction: kurangi stok tiap item (atomik, hanya jika stok cukup),
// tambah akumulasi penjualan bulanan, lalu simpan order. Semua dalam 1 transaksi:
// jika ada satu saja barang yang kurang, seluruh pesanan dibatalkan (rollback).
func (r *OrderRepo) CreateWithTransaction(ctx context.Context, order *domain.Order) error {
	year, month := periodOf(order.CreatedAt)

	var shortages []domain.InsufficientStockDetail

	err := r.withTx(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		// callback bisa dijalankan ulang oleh driver (WriteConflict), jadi reset.
		shortages = shortages[:0]
		now := time.Now()

		for _, it := range order.Items {
			res, err := r.products.UpdateOne(sc,
				bson.M{
					"kode_barang": it.ProductCode,
					"is_active":   true,
					"stock":       bson.M{"$gte": it.Qty},
				},
				bson.M{
					"$inc": bson.M{"stock": -it.Qty},
					"$set": bson.M{"updated_at": now},
				},
			)
			if err != nil {
				return nil, err
			}
			if res.MatchedCount == 0 {
				// catat semua barang yang kurang (AC3), lanjut cek item berikutnya.
				shortages = append(shortages, domain.InsufficientStockDetail{
					ProductCode: it.ProductCode,
					Requested:   it.Qty,
					Available:   r.availableStock(sc, it.ProductCode),
				})
				continue
			}

			// akumulasi riwayat bulanan (upsert + $inc) -> dipakai analitik & forecast
			_, err = r.history.UpdateOne(sc,
				bson.M{"product_code": it.ProductCode, "year": year, "month": month},
				bson.M{
					"$inc":         bson.M{"qty_sold": it.Qty, "revenue": it.Subtotal},
					"$set":         bson.M{"updated_at": now},
					"$setOnInsert": bson.M{"source": "live"},
				},
				options.Update().SetUpsert(true),
			)
			if err != nil {
				return nil, err
			}
		}

		if len(shortages) > 0 {
			return nil, errInsufficientStock // abort -> semua perubahan di-rollback
		}

		ins, err := r.orders.InsertOne(sc, order)
		if err != nil {
			return nil, err
		}
		if oid, ok := ins.InsertedID.(primitive.ObjectID); ok {
			order.ID = oid
		}
		return nil, nil
	})

	if errors.Is(err, errInsufficientStock) {
		return domain.NewInsufficientStockError("Stok tidak mencukupi", shortages...)
	}
	return err
}

// FindAll: riwayat pesanan dengan filter nim, rentang tanggal, status; terbaru dulu.
func (r *OrderRepo) FindAll(ctx context.Context, f domain.OrderFilter) ([]domain.Order, int64, error) {
	q := bson.M{}
	if f.MemberNIM != "" {
		q["member_nim"] = f.MemberNIM
	}
	if f.Status != "" {
		q["status"] = f.Status
	}
	switch f.Channel {
	case domain.ChannelOnline:
		q["channel"] = domain.ChannelOnline
	case domain.ChannelOffline:
		// pesanan lama tidak punya field channel -> dianggap OFFLINE
		q["channel"] = bson.M{"$ne": domain.ChannelOnline}
	}
	if f.From != nil || f.To != nil {
		rng := bson.M{}
		if f.From != nil {
			rng["$gte"] = *f.From
		}
		if f.To != nil {
			rng["$lte"] = *f.To
		}
		q["created_at"] = rng
	}

	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	total, err := r.orders.CountDocuments(ctx, q)
	if err != nil {
		return nil, 0, err
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))

	cur, err := r.orders.Find(ctx, q, findOpts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	orders := make([]domain.Order, 0, limit) // non-nil -> JSON jadi [] bukan null
	if err := cur.All(ctx, &orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *OrderRepo) FindByOrderNo(ctx context.Context, orderNo string) (*domain.Order, error) {
	var o domain.Order
	err := r.orders.FindOne(ctx, bson.M{"order_no": orderNo}).Decode(&o)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("Pesanan tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// CancelWithTransaction: ubah status COMPLETED -> CANCELLED, kembalikan stok, dan
// kurangi akumulasi penjualan bulanan (bulan saat pesanan dibuat) dalam 1 transaksi.
func (r *OrderRepo) CancelWithTransaction(ctx context.Context, orderNo string) error {
	return r.withTx(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		now := time.Now()

		// Balik status secara atomik. Hanya order COMPLETED yang cocok, sehingga
		// dua pembatalan bersamaan tidak akan mengembalikan stok dua kali.
		var order domain.Order
		err := r.orders.FindOneAndUpdate(sc,
			bson.M{"order_no": orderNo, "status": domain.OrderCompleted},
			bson.M{"$set": bson.M{"status": domain.OrderCancelled}},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&order)

		if errors.Is(err, mongo.ErrNoDocuments) {
			n, cerr := r.orders.CountDocuments(sc, bson.M{"order_no": orderNo})
			if cerr != nil {
				return nil, cerr
			}
			if n == 0 {
				return nil, domain.NewNotFoundError("Pesanan tidak ditemukan")
			}
			return nil, domain.NewAppError(http.StatusConflict, "ALREADY_CANCELLED",
				"Pesanan sudah dibatalkan", nil)
		}
		if err != nil {
			return nil, err
		}

		year, month := periodOf(order.CreatedAt)

		for _, it := range order.Items {
			if _, err := r.products.UpdateOne(sc,
				bson.M{"kode_barang": it.ProductCode},
				bson.M{
					"$inc": bson.M{"stock": it.Qty},
					"$set": bson.M{"updated_at": now},
				},
			); err != nil {
				return nil, err
			}

			if _, err := r.history.UpdateOne(sc,
				bson.M{"product_code": it.ProductCode, "year": year, "month": month},
				bson.M{
					"$inc": bson.M{"qty_sold": -it.Qty, "revenue": -it.Subtotal},
					"$set": bson.M{"updated_at": now},
				},
			); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
}
