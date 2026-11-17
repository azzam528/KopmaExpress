package mongo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"smartcoop/internal/domain"
)

type MemberRepo struct {
	collection *mongo.Collection
	orders     *mongo.Collection
}

func NewMemberRepo(db *mongo.Database) *MemberRepo {
	return &MemberRepo{
		collection: db.Collection("members"),
		orders:     db.Collection("orders"),
	}
}

func (r *MemberRepo) FindByNIM(ctx context.Context, nim string) (*domain.Member, error) {
	var member domain.Member

	err := r.collection.FindOne(ctx, bson.M{
		"nim": nim,
	}).Decode(&member)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("Anggota tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	return &member, nil
}

// FindAll: pencarian NIM/nama (memuat teks, tanpa membedakan huruf besar/kecil),
// filter status, dan paging. Anggota aktif tampil lebih dulu, lalu urut nama.
func (r *MemberRepo) FindAll(ctx context.Context, f domain.MemberFilter) ([]domain.Member, int64, error) {
	q := bson.M{}

	if f.Query != "" {
		// QuoteMeta: input pengguna diperlakukan sebagai teks biasa, bukan regex.
		re := primitive.Regex{Pattern: regexp.QuoteMeta(f.Query), Options: "i"}
		q["$or"] = bson.A{bson.M{"nim": re}, bson.M{"name": re}}
	}

	switch f.Status {
	case domain.MemberStatusActive:
		q["is_active"] = true
	case domain.MemberStatusInactive:
		q["is_active"] = false
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

	total, err := r.collection.CountDocuments(ctx, q)
	if err != nil {
		return nil, 0, err
	}

	cur, err := r.collection.Find(ctx, q, options.Find().
		SetSort(bson.D{
			{Key: "is_active", Value: -1},
			{Key: "name", Value: 1},
			{Key: "_id", Value: 1},
		}).
		SetSkip(int64((page-1)*limit)).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	members := make([]domain.Member, 0, limit) // non-nil -> JSON [] bukan null
	if err := cur.All(ctx, &members); err != nil {
		return nil, 0, err
	}
	return members, total, nil
}

// Create menyimpan anggota baru. NIM ganda dikembalikan sebagai error E11000
// (index unik idx_members_nim_unique); service yang mengubahnya jadi 409.
func (r *MemberRepo) Create(ctx context.Context, m *domain.Member) error {
	res, err := r.collection.InsertOne(ctx, m)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		m.ID = oid
	}
	return nil
}

// Update mengubah nama, unit, dan telepon. NIM tidak bisa diubah.
func (r *MemberRepo) Update(ctx context.Context, nim string, m *domain.Member) error {
	res, err := r.collection.UpdateOne(ctx,
		bson.M{"nim": nim},
		bson.M{"$set": bson.M{
			"name":  m.Name,
			"unit":  m.Unit,
			"phone": m.Phone,
		}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.NewNotFoundError("Anggota tidak ditemukan")
	}
	return nil
}

func (r *MemberRepo) Deactivate(ctx context.Context, nim string) error {
	return r.setActive(ctx, nim, false)
}

func (r *MemberRepo) Activate(ctx context.Context, nim string) error {
	return r.setActive(ctx, nim, true)
}

func (r *MemberRepo) setActive(ctx context.Context, nim string, active bool) error {
	res, err := r.collection.UpdateOne(ctx,
		bson.M{"nim": nim},
		bson.M{"$set": bson.M{"is_active": active}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.NewNotFoundError("Anggota tidak ditemukan")
	}
	return nil
}

// DeletePermanent menghapus anggota dari koleksi. Syarat: anggota sudah nonaktif
// (anggota nonaktif tidak bisa membuat pesanan, jadi jumlah pesanannya tidak
// bisa bertambah di antara pengecekan dan penghapusan) dan belum punya pesanan
// berstatus apa pun, supaya riwayat transaksi tidak menjadi yatim.
func (r *MemberRepo) DeletePermanent(ctx context.Context, nim string) error {
	m, err := r.FindByNIM(ctx, nim) // 404 bila tidak ada
	if err != nil {
		return err
	}
	if m.IsActive {
		return domain.NewAppError(http.StatusConflict, domain.CodeMemberActive,
			"Nonaktifkan anggota terlebih dahulu sebelum menghapus permanen", nil)
	}

	n, err := r.orders.CountDocuments(ctx, bson.M{"member_nim": nim})
	if err != nil {
		return err
	}
	if n > 0 {
		return domain.NewAppError(http.StatusConflict, domain.CodeMemberHasOrders,
			fmt.Sprintf("Anggota punya %d pesanan, nonaktifkan saja", n), nil)
	}

	// filter is_active:false menjaga dari anggota yang diaktifkan kembali di sela waktu itu
	res, err := r.collection.DeleteOne(ctx, bson.M{"nim": nim, "is_active": false})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return domain.NewAppError(http.StatusConflict, domain.CodeMemberActive,
			"Status anggota berubah, muat ulang halaman lalu coba lagi", nil)
	}
	return nil
}

// Summary menghitung angka ringkasan. Pembeli aktif = jumlah NIM berbeda pada
// pesanan COMPLETED sejak buyersSince.
func (r *MemberRepo) Summary(ctx context.Context, buyersSince time.Time) (*domain.MemberSummary, error) {
	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	active, err := r.collection.CountDocuments(ctx, bson.M{"is_active": true})
	if err != nil {
		return nil, err
	}

	buyers, err := r.orders.Distinct(ctx, "member_nim", bson.M{
		"status":     domain.OrderCompleted,
		"created_at": bson.M{"$gte": buyersSince},
	})
	if err != nil {
		return nil, err
	}

	return &domain.MemberSummary{
		Total:          total,
		Active:         active,
		Inactive:       total - active,
		ActiveBuyers30: int64(len(buyers)),
	}, nil
}
