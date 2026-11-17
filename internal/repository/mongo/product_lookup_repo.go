package mongo

import (
	"context"
	"regexp"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"smartcoop/internal/domain"
)

// ProductLookupRepo: pencarian barang SEMENTARA untuk pemilih barang di halaman
// pesanan (Orang 3). Hapus file ini bila repo produk lengkap milik Orang 1
// sudah punya FindAll.
type ProductLookupRepo struct {
	col *mongo.Collection
}

func NewProductLookupRepo(db *mongo.Database) *ProductLookupRepo {
	return &ProductLookupRepo{col: db.Collection("products")}
}

// Search mengembalikan barang AKTIF yang nama atau kodenya memuat q
// (tanpa membedakan huruf besar/kecil), urut nama. q kosong = semua barang.
func (r *ProductLookupRepo) Search(ctx context.Context, q string, limit int) ([]domain.Product, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	filter := bson.M{"is_active": true}
	if q != "" {
		// QuoteMeta: input pengguna diperlakukan sebagai teks biasa, bukan regex.
		re := primitive.Regex{Pattern: regexp.QuoteMeta(q), Options: "i"}
		filter["$or"] = bson.A{bson.M{"name": re}, bson.M{"kode_barang": re}}
	}

	cur, err := r.col.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	products := make([]domain.Product, 0, limit) // non-nil -> JSON [] bukan null
	if err := cur.All(ctx, &products); err != nil {
		return nil, err
	}
	return products, nil
}
