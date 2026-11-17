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

// MemberLookupRepo: pencarian anggota SEMENTARA untuk pemilih NIM di halaman
// pesanan (Orang 3). Hapus file ini bila repo anggota milik Orang 1 sudah
// punya pencarian sendiri.
type MemberLookupRepo struct {
	col *mongo.Collection
}

func NewMemberLookupRepo(db *mongo.Database) *MemberLookupRepo {
	return &MemberLookupRepo{col: db.Collection("members")}
}

// Search mengembalikan anggota yang NIM atau namanya memuat q (tanpa
// membedakan huruf besar/kecil). Anggota non-aktif ikut dikembalikan supaya
// halaman bisa menandainya, tetapi diurutkan paling akhir. q kosong = semua.
func (r *MemberLookupRepo) Search(ctx context.Context, q string, limit int) ([]domain.Member, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	filter := bson.M{}
	if q != "" {
		// QuoteMeta: input pengguna diperlakukan sebagai teks biasa, bukan regex.
		re := primitive.Regex{Pattern: regexp.QuoteMeta(q), Options: "i"}
		filter["$or"] = bson.A{bson.M{"nim": re}, bson.M{"name": re}}
	}

	cur, err := r.col.Find(ctx, filter,
		options.Find().
			SetSort(bson.D{{Key: "is_active", Value: -1}, {Key: "name", Value: 1}}).
			SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	members := make([]domain.Member, 0, limit) // non-nil -> JSON [] bukan null
	if err := cur.All(ctx, &members); err != nil {
		return nil, err
	}
	return members, nil
}
