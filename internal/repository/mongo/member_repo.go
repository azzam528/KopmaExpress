package mongo

import (
	"context"
	"errors"

	"smartcoop/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type MemberRepo struct {
	collection *mongo.Collection
}

func NewMemberRepo(db *mongo.Database) *MemberRepo {
	return &MemberRepo{
		collection: db.Collection("members"),
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
