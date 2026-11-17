package mongo

import (
	"context"
	"errors"
	"time"

	"smartcoop/internal/domain"
	"smartcoop/internal/etl"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ProductRepo struct {
	collection *mongo.Collection
}

func NewProductRepo(db *mongo.Database) *ProductRepo {
	return &ProductRepo{
		collection: db.Collection("products"),
	}
}

func (r *ProductRepo) FindAll(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int64, error) {
	q := bson.M{}
	if filter.Query != "" {
		q["$or"] = bson.A{
			bson.M{"kode_barang": bson.M{"$regex": filter.Query, "$options": "i"}},
			bson.M{"name": bson.M{"$regex": filter.Query, "$options": "i"}},
		}
	}
	if filter.Category != "" {
		q["category"] = filter.Category
	}
	if filter.IsActive != nil {
		q["is_active"] = *filter.IsActive
	}

	total, err := r.collection.CountDocuments(ctx, q)
	if err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	sortField := "name"
	sortDir := 1
	if filter.Sort == "stock_desc" {
		sortField, sortDir = "stock", -1
	} else if filter.Sort == "price_asc" {
		sortField, sortDir = "price", 1
	} else if filter.Sort == "price_desc" {
		sortField, sortDir = "price", -1
	}

	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: sortField, Value: sortDir}})

	cursor, err := r.collection.Find(ctx, q, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var products []domain.Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *ProductRepo) FindByCode(ctx context.Context, code string) (*domain.Product, error) {
	var product domain.Product
	err := r.collection.FindOne(ctx, bson.M{"kode_barang": code}).Decode(&product)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("Produk tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepo) Create(ctx context.Context, p *domain.Product) error {
	now := time.Now()
	p.ID = primitive.NewObjectID()
	p.CreatedAt = now
	p.UpdatedAt = now
	_, err := r.collection.InsertOne(ctx, p)
	return err
}

func (r *ProductRepo) Update(ctx context.Context, code string, p *domain.Product) error {
	p.UpdatedAt = time.Now()
	update := bson.M{
		"$set": bson.M{
			"name":       p.Name,
			"category":   p.Category,
			"price":      p.Price,
			"stock":      p.Stock,
			"is_active":  p.IsActive,
			"updated_at": p.UpdatedAt,
		},
	}
	res, err := r.collection.UpdateOne(ctx, bson.M{"kode_barang": code}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.NewNotFoundError("Produk tidak ditemukan")
	}
	return nil
}

func (r *ProductRepo) SoftDelete(ctx context.Context, code string) error {
	res, err := r.collection.UpdateOne(
		ctx,
		bson.M{"kode_barang": code},
		bson.M{"$set": bson.M{"is_active": false, "updated_at": time.Now()}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.NewNotFoundError("Produk tidak ditemukan")
	}
	return nil
}

// BulkUpsert melakukan upsert batch produk dari Excel.
//
//   - stockMode "replace" → stok di-set ke nilai file
//   - stockMode "skip"    → stok tidak diubah jika produk sudah ada
//
// Mengembalikan jumlah dokumen yang baru dibuat (inserted) dan yang diperbarui (updated).

func (r *ProductRepo) BulkUpsert(ctx context.Context, products []domain.Product, stockMode string) (inserted int64, updated int64, err error) {
	if len(products) == 0 {
		return 0, 0, nil
	}

	now := time.Now()
	models := make([]mongo.WriteModel, 0, len(products))

	for _, p := range products {
		filter := bson.M{"kode_barang": p.Code}

		set := bson.M{
			"name":       p.Name,
			"category":   p.Category,
			"price":      p.Price,
			"is_active":  true,
			"updated_at": now,
		}

		setOnInsert := bson.M{
			"kode_barang": p.Code,
			"created_at":  now,
		}

		if stockMode == etl.StockModeSkip {
			// Produk baru mendapatkan stok dari Excel.
			// Produk lama mempertahankan stok sebelumnya.
			setOnInsert["stock"] = p.Stock
		} else {
			// Replace: stok selalu mengikuti nilai dari Excel.
			set["stock"] = p.Stock
		}

		update := bson.M{
			"$set":         set,
			"$setOnInsert": setOnInsert,
		}

		model := mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true)

		models = append(models, model)
	}

	opts := options.BulkWrite().SetOrdered(false)

	res, err := r.collection.BulkWrite(ctx, models, opts)
	if err != nil {
		return 0, 0, err
	}

	return res.UpsertedCount, res.ModifiedCount, nil
}
