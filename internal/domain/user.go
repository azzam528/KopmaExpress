package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User Roles
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// User Statuses
const (
	UserStatusActive   = "active"
	UserStatusInactive = "inactive"
)

// User represents the identity account for authentication and authorization.
// It is stored in the "users" collection, separate from "members".
type User struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	NIM          string              `bson:"nim" json:"nim" validate:"required"`
	Name         string              `bson:"name" json:"name" validate:"required"`
	PasswordHash string              `bson:"password_hash" json:"-"`
	Role         string              `bson:"role" json:"role" validate:"required"`
	Status       string              `bson:"status" json:"status"`
	MemberID     *primitive.ObjectID `bson:"member_id,omitempty" json:"member_id,omitempty"`
	CreatedAt    time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time           `bson:"updated_at" json:"updated_at"`
}

// UserProfileResponse is the public profile payload returned by /api/v1/auth/me
type UserProfileResponse struct {
	UserID   string      `json:"user_id"`
	NIM      string      `json:"nim"`
	Name     string      `json:"name"`
	Role     string      `json:"role"`
	Status   string      `json:"status"`
	MemberID *string     `json:"member_id,omitempty"`
	Member   *MemberInfo `json:"member,omitempty"`
}

// MemberInfo represents the embedded cooperative member profile in UserProfileResponse
type MemberInfo struct {
	ID       string `json:"id"`
	NIM      string `json:"nim"`
	Name     string `json:"nama"`
	Unit     string `json:"unit,omitempty"`
	Phone    string `json:"telepon,omitempty"`
	Email    string `json:"email,omitempty"`
	IsActive bool   `json:"is_active"`
}

// UserRepository defines the persistence contract for User accounts in MongoDB.
type UserRepository interface {
	FindByNIM(ctx context.Context, nim string) (*User, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*User, error)
	FindByMemberID(ctx context.Context, memberID primitive.ObjectID) (*User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, id primitive.ObjectID, u *User) error
	Count(ctx context.Context) (int64, error)
}
