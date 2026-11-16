package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Product represents the product catalog entity (§4.2)
type Product struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"  json:"id"`
	Code      string             `bson:"kode_barang"    json:"kode_barang" validate:"required,max=30"`
	Name      string             `bson:"name"           json:"nama_barang" validate:"required,max=150"`
	Category  string             `bson:"category"       json:"kategori"    validate:"required,max=60"`
	Price     int64              `bson:"price"          json:"harga"       validate:"gte=0"` // Rupiah (integer)
	Stock     int64              `bson:"stock"          json:"stok"        validate:"gte=0"`
	IsActive  bool               `bson:"is_active"      json:"is_active"`
	CreatedAt time.Time          `bson:"created_at"     json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"     json:"updated_at"`
}

type ProductFilter struct {
	Query    string
	Category string
	IsActive *bool
	Page     int
	Limit    int
	Sort     string
}

type CreateProductRequest struct {
	Code     string `json:"kode_barang"`
	Name     string `json:"nama_barang"`
	Category string `json:"kategori"`
	Price    int64  `json:"harga"`
	Stock    int64  `json:"stok"`
}

type UpdateProductRequest struct {
	Name     string `json:"nama_barang"`
	Category string `json:"kategori"`
	Price    *int64 `json:"harga"`
	Stock    *int64 `json:"stok"`
}

type ProductRepository interface {
	FindAll(ctx context.Context, filter ProductFilter) ([]Product, int64, error)
	FindByCode(ctx context.Context, code string) (*Product, error)
	Create(ctx context.Context, p *Product) error
	Update(ctx context.Context, code string, p *Product) error
	SoftDelete(ctx context.Context, code string) error
	BulkUpsert(ctx context.Context, products []Product, stockMode string) (inserted int64, updated int64, err error)
}

type ProductService interface {
	List(ctx context.Context, filter ProductFilter) ([]Product, int64, error)
	GetByCode(ctx context.Context, code string) (*Product, error)
	Create(ctx context.Context, req *CreateProductRequest) (*Product, error)
	Update(ctx context.Context, code string, req *UpdateProductRequest) (*Product, error)
	Delete(ctx context.Context, code string) error
}
