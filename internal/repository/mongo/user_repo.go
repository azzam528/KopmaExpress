package mongo

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"smartcoop/internal/domain"
)

type UserRepo struct {
	collection *mongo.Collection
}

var _ domain.UserRepository = (*UserRepo)(nil)

func NewUserRepo(db *mongo.Database) *UserRepo {
	return &UserRepo{
		collection: db.Collection("users"),
	}
}

func (r *UserRepo) FindByNIM(ctx context.Context, nim string) (*domain.User, error) {
	normNIM := strings.TrimSpace(nim)
	if normNIM == "" {
		return nil, domain.NewNotFoundError("User tidak ditemukan")
	}

	var user domain.User
	err := r.collection.FindOne(ctx, bson.M{
		"nim": normNIM,
	}).Decode(&user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("User tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.User, error) {
	var user domain.User
	err := r.collection.FindOne(ctx, bson.M{
		"_id": id,
	}).Decode(&user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("User tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) FindByMemberID(ctx context.Context, memberID primitive.ObjectID) (*domain.User, error) {
	var user domain.User
	err := r.collection.FindOne(ctx, bson.M{
		"member_id": memberID,
	}).Decode(&user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("User tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	now := time.Now()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	u.NIM = strings.TrimSpace(u.NIM)

	res, err := r.collection.InsertOne(ctx, u)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.NewDuplicateError("NIM akun sudah digunakan")
		}
		return err
	}

	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		u.ID = oid
	}
	return nil
}

func (r *UserRepo) Update(ctx context.Context, id primitive.ObjectID, u *domain.User) error {
	u.UpdatedAt = time.Now()

	update := bson.M{
		"$set": bson.M{
			"name":       u.Name,
			"role":       u.Role,
			"status":     u.Status,
			"member_id":  u.MemberID,
			"updated_at": u.UpdatedAt,
		},
	}
	if u.PasswordHash != "" {
		update["$set"].(bson.M)["password_hash"] = u.PasswordHash
	}

	res, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.NewNotFoundError("User tidak ditemukan")
	}
	return nil
}

func (r *UserRepo) Count(ctx context.Context) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{})
}
