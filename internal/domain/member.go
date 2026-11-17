
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

type MemberFilter struct {
	Query string
	Page  int
	Limit int
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
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateMemberRequest struct {
	Name  string `json:"nama"`
	Unit  string `json:"unit"`
	Phone string `json:"telepon"`
}

type MemberRepository interface {
	FindAll(ctx context.Context, filter MemberFilter) ([]Member, int64, error)
	FindByNIM(ctx context.Context, nim string) (*Member, error)
	FindByEmail(ctx context.Context, email string) (*Member, error)
	Create(ctx context.Context, m *Member) error
	Update(ctx context.Context, nim string, m *Member) error
	Deactivate(ctx context.Context, nim string) error
	BulkUpsert(ctx context.Context, members []Member) (inserted int64, updated int64, err error)
}

type MemberService interface {
	List(ctx context.Context, filter MemberFilter) ([]Member, int64, error)
	GetByNIM(ctx context.Context, nim string) (*Member, error)
	Create(ctx context.Context, req *CreateMemberRequest) (*Member, error)
	Update(ctx context.Context, nim string, req *UpdateMemberRequest) (*Member, error)
	Delete(ctx context.Context, nim string) error
}

