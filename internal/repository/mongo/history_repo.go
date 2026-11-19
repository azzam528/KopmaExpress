package mongo

import (
	"context"
	"strings"
	"time"

	"smartcoop/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type HistoryRepo struct {
	collection *mongo.Collection
}

func NewHistoryRepo(db *mongo.Database) *HistoryRepo {
	return &HistoryRepo{
		collection: db.Collection("monthly_sales_history"),
	}
}

// UpsertSales menyimpan satu record penjualan bulanan.
// Jika sudah ada (product_code + year + month sama), data akan digantikan.
func (r *HistoryRepo) UpsertSales(ctx context.Context, item *domain.MonthlySalesHistory) error {
	filter := bson.M{
		"product_code": item.ProductCode,
		"year":         item.Year,
		"month":        item.Month,
	}
	update := bson.M{
		"$set": bson.M{
			"qty_sold":   item.QtySold,
			"revenue":    item.Revenue,
			"source":     item.Source,
			"updated_at": time.Now(),
		},
		"$setOnInsert": bson.M{
			"product_code": item.ProductCode,
			"year":         item.Year,
			"month":        item.Month,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// BulkUpsert melakukan upsert batch riwayat penjualan dari Excel.
// Conflict key: (product_code, year, month) — jika sudah ada, qty_sold dan revenue diperbarui.
// Mengembalikan jumlah dokumen baru (inserted) dan yang diperbarui (updated).
func (r *HistoryRepo) BulkUpsert(ctx context.Context, records []domain.MonthlySalesHistory) (inserted int64, updated int64, err error) {
	if len(records) == 0 {
		return 0, 0, nil
	}

	now := time.Now()
	models := make([]mongo.WriteModel, 0, len(records))

	for _, h := range records {
		code := strings.ToUpper(strings.TrimSpace(h.ProductCode))
		if code == "" {
			continue
		}
		filter := bson.M{
			"product_code": code,
			"year":         h.Year,
			"month":        h.Month,
		}
		update := bson.M{
			"$set": bson.M{
				"qty_sold":   h.QtySold,
				"revenue":    h.Revenue,
				"source":     h.Source,
				"updated_at": now,
			},
			"$setOnInsert": bson.M{
				"product_code": code,
				"year":         h.Year,
				"month":        h.Month,
			},
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

func (r *HistoryRepo) FindByProduct(ctx context.Context, productCode string) ([]domain.MonthlySalesHistory, error) {
	opts := options.Find().SetSort(bson.D{
		{Key: "year", Value: 1},
		{Key: "month", Value: 1},
	})
	cursor, err := r.collection.Find(ctx, bson.M{"product_code": productCode}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var records []domain.MonthlySalesHistory
	if err := cursor.All(ctx, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *HistoryRepo) GetTopSelling(ctx context.Context, fromYear, fromMonth, limit int) ([]domain.TopSellingProduct, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"$or": bson.A{
				bson.M{"year": bson.M{"$gt": fromYear}},
				bson.M{"year": fromYear, "month": bson.M{"$gte": fromMonth}},
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":         "$product_code",
			"total_qty":   bson.M{"$sum": "$qty_sold"},
			"total_omzet": bson.M{"$sum": "$revenue"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "total_qty", Value: -1}}}},
		{{Key: "$limit", Value: limit}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "products",
			"localField":   "_id",
			"foreignField": "kode_barang",
			"as":           "product",
		}}},
		{{Key: "$unwind", Value: bson.M{"path": "$product", "preserveNullAndEmptyArrays": true}}},
		{{Key: "$project", Value: bson.M{
			"_id":         0,
			"kode_barang": "$_id",
			"nama_barang": "$product.name",
			"kategori":    "$product.category",
			"total_qty":   1,
			"total_omzet": 1,
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []domain.TopSellingProduct
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Rank = i + 1
	}
	return results, nil
}

func (r *HistoryRepo) GetMonthlyTrend(ctx context.Context, productCode string) ([]domain.MonthlyTrendItem, error) {
	opts := options.Find().SetSort(bson.D{
		{Key: "year", Value: 1},
		{Key: "month", Value: 1},
	})
	cursor, err := r.collection.Find(ctx, bson.M{"product_code": productCode}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []domain.MonthlyTrendItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}
