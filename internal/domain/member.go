
package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Member represents the cooperative member entity (§4.2)
type Member struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	NIM          string             `bson:"nim" json:"nim" validate:"required,max=20"`
	Name         string             `bson:"name" json:"nama" validate:"required,max=120"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	Unit         string             `bson:"unit" json:"unit"`
	Phone        string             `bson:"phone" json:"telepon"`
	IsActive     bool               `bson:"is_active" json:"is_active"`
	JoinedAt     time.Time          `bson:"joined_at" json:"joined_at"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

// Kode error khusus penghapusan permanen anggota (HTTP 409).
const (
	CodeMemberActive    = "MEMBER_ACTIVE"
	CodeMemberHasOrders = "MEMBER_HAS_ORDERS"
)

// Nilai filter status pada daftar anggota ("" = semua).
const (
	MemberStatusActive   = "active"
	MemberStatusInactive = "inactive"
)

type MemberFilter struct {
	Query  string
	Status string // "", MemberStatusActive, atau MemberStatusInactive
	Page   int
	Limit  int
}

// MemberSummary adalah ringkasan angka untuk halaman Master Anggota.
type MemberSummary struct {
	Total          int64 `json:"total"`
	Active         int64 `json:"aktif"`
	Inactive       int64 `json:"nonaktif"`
	ActiveBuyers30 int64 `json:"bertransaksi_30_hari"`
}

type CreateMemberRequest struct {
	NIM      string     `json:"nim"`
	Name     string     `json:"nama"`
	Email    string     `json:"email"`
	Password string     `json:"password"`
	Unit     string     `json:"unit"`
	Phone    string     `json:"telepon"`
	JoinedAt *time.Time `json:"joined_at,omitempty"`
}

type LoginRequest struct {
	NIM      string `json:"nim"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password"`
}

type UpdateMemberRequest struct {
	// NIM baru (opsional). Kosong atau sama dengan NIM lama = tidak berubah.
	// Bila berubah, NIM pada seluruh pesanan anggota ini ikut diperbarui.
	NIM   string `json:"nim"`
	Name  string `json:"nama"`
	Unit  string `json:"unit"`
	Phone string `json:"telepon"`
}

type MemberRepository interface {
	FindAll(ctx context.Context, filter MemberFilter) ([]Member, int64, error)
	FindByNIM(ctx context.Context, nim string) (*Member, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*Member, error)
	FindByEmail(ctx context.Context, email string) (*Member, error)
	Create(ctx context.Context, m *Member) error
	Update(ctx context.Context, nim string, m *Member) error
	Deactivate(ctx context.Context, nim string) error
	Activate(ctx context.Context, nim string) error
	// DeletePermanent menghapus anggota dari database. Hanya boleh bila anggota
	// sudah nonaktif dan belum punya satu pun pesanan (409 bila tidak).
	DeletePermanent(ctx context.Context, nim string) error
	// Summary menghitung total/aktif/nonaktif, serta anggota yang punya pesanan
	// COMPLETED sejak buyersSince.
	Summary(ctx context.Context, buyersSince time.Time) (*MemberSummary, error)
	BulkUpsert(ctx context.Context, members []Member) (inserted int64, updated int64, err error)
}

type MemberService interface {
	List(ctx context.Context, filter MemberFilter) ([]Member, int64, error)
	GetByNIM(ctx context.Context, nim string) (*Member, error)
	Create(ctx context.Context, req *CreateMemberRequest) (*Member, error)
	Update(ctx context.Context, nim string, req *UpdateMemberRequest) (*Member, error)
	Delete(ctx context.Context, nim string) error // menonaktifkan, bukan menghapus
	Activate(ctx context.Context, nim string) error
	DeletePermanent(ctx context.Context, nim string) error
	Summary(ctx context.Context) (*MemberSummary, error)
}

