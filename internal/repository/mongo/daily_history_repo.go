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

// DailyHistoryRepo menyimpan penjualan harian hasil import Excel.
type DailyHistoryRepo struct {
	collection *mongo.Collection
}

func NewDailyHistoryRepo(db *mongo.Database) *DailyHistoryRepo {
	return &DailyHistoryRepo{collection: db.Collection("daily_sales_history")}
}

// BulkUpsert melakukan upsert batch. Kunci (product_code, date): jika sudah
// ada, qty_sold dan revenue ditimpa.
func (r *DailyHistoryRepo) BulkUpsert(ctx context.Context, records []domain.DailySalesHistory) (inserted int64, updated int64, err error) {
	if len(records) == 0 {
		return 0, 0, nil
	}

	now := time.Now()
	models := make([]mongo.WriteModel, 0, len(records))
	for _, h := range records {
		code := strings.ToUpper(strings.TrimSpace(h.ProductCode))
		if code == "" || h.Date == "" {
			continue
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"product_code": code, "date": h.Date}).
			SetUpdate(bson.M{
				"$set": bson.M{
					"qty_sold":   h.QtySold,
					"revenue":    h.Revenue,
					"source":     h.Source,
					"updated_at": now,
				},
				"$setOnInsert": bson.M{
					"product_code": code,
					"date":         h.Date,
				},
			}).
			SetUpsert(true))
	}
	if len(models) == 0 {
		return 0, 0, nil
	}

	res, err := r.collection.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, 0, err
	}
	return res.UpsertedCount, res.ModifiedCount, nil
}
