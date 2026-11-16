package mongo

import (
	"context"
	"errors"

	"smartcoop/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProductRepo struct {
	collection *mongo.Collection
}

func NewProductRepo(db *mongo.Database) *ProductRepo {
	return &ProductRepo{
		collection: db.Collection("products"),
	}
}

func (r *ProductRepo) FindByCode(ctx context.Context, code string) (*domain.Product, error) {
	var product domain.Product

	err := r.collection.FindOne(ctx, bson.M{
		"kode_barang": code,
	}).Decode(&product)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("Produk tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	return &product, nil
}
