package mongo

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"smartcoop/internal/domain"
	"smartcoop/internal/timeutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ForecastRepo persists forecast results and loads each active product's sales history.
type ForecastRepo struct {
	db *mongo.Database
}

func NewForecastRepo(db *mongo.Database) *ForecastRepo {
	return &ForecastRepo{db: db}
}

func (r *ForecastRepo) UpsertForecasts(ctx context.Context, results []domain.ForecastResult) error {
	if len(results) == 0 {
		return nil
	}

	models := make([]mongo.WriteModel, 0, len(results))
	for _, result := range results {
		encoded, err := bson.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal forecast for %s: %w", result.ProductCode, err)
		}
		var fields bson.M
		if err := bson.Unmarshal(encoded, &fields); err != nil {
			return fmt.Errorf("decode forecast for %s: %w", result.ProductCode, err)
		}
		delete(fields, "_id")

		model := mongo.NewUpdateOneModel().
			SetFilter(bson.M{
				"product_code": result.ProductCode,
				"target_month": result.TargetMonth,
			}).
			SetUpdate(bson.M{"$set": fields}).
			SetUpsert(true)
		models = append(models, model)
	}

	_, err := r.db.Collection("forecast_results").BulkWrite(
		ctx,
		models,
		options.BulkWrite().SetOrdered(false),
	)
	if err != nil {
		return fmt.Errorf("bulk upsert forecast results: %w", err)
	}

		codes := make([]string, 0, len(results))
	for _, result := range results {
		codes = append(codes, result.ProductCode)
	}
	if _, err := r.db.Collection("forecast_results").DeleteMany(ctx, bson.M{
		"target_month": results[0].TargetMonth,
		"product_code": bson.M{"$nin": codes},
	}); err != nil {
		return fmt.Errorf("remove stale forecast results: %w", err)
	}
	return nil
}

func (r *ForecastRepo) FindAll(ctx context.Context, filter domain.ForecastFilter) ([]domain.ForecastResult, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	now := timeutil.Now()
	targetMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01")
	query := bson.M{"target_month": targetMonth}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.Query != "" {
		pattern := regexp.QuoteMeta(filter.Query)
		query["$or"] = bson.A{
			bson.M{"product_code": bson.M{"$regex": pattern, "$options": "i"}},
			bson.M{"product_name": bson.M{"$regex": pattern, "$options": "i"}},
		}
	}

	collection := r.db.Collection("forecast_results")
	total, err := collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count forecast results: %w", err)
	}

	findOptions := options.Find().
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit)).
		SetSort(forecastSort(filter.Sort))
	cursor, err := collection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("find forecast results: %w", err)
	}
	defer cursor.Close(ctx)

	results := make([]domain.ForecastResult, 0)
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, fmt.Errorf("decode forecast results: %w", err)
	}
	return results, total, nil
}

func (r *ForecastRepo) FindByProductCode(ctx context.Context, code string) (*domain.ForecastResult, error) {
	var result domain.ForecastResult
	err := r.db.Collection("forecast_results").FindOne(
		ctx,
		bson.M{"product_code": code},
		options.FindOne().SetSort(bson.D{{Key: "target_month", Value: -1}, {Key: "calculated_at", Value: -1}}),
	).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, domain.NewNotFoundError("Hasil forecast untuk barang tersebut belum tersedia")
		}
		return nil, err
	}
	return &result, nil
}

func (r *ForecastRepo) FindSalesHistory(ctx context.Context, code string) ([]domain.MonthlySalesHistory, error) {
	findOptions := options.Find().SetSort(bson.D{{Key: "year", Value: 1}, {Key: "month", Value: 1}})
	cursor, err := r.db.Collection("monthly_sales_history").Find(ctx, bson.M{"product_code": code}, findOptions)
	if err != nil {
		return nil, fmt.Errorf("find sales history for %s: %w", code, err)
	}
	defer cursor.Close(ctx)

	history := make([]domain.MonthlySalesHistory, 0)
	if err := cursor.All(ctx, &history); err != nil {
		return nil, fmt.Errorf("decode sales history for %s: %w", code, err)
	}
	return history, nil
}

func (r *ForecastRepo) GetStatusSummary(ctx context.Context) (map[domain.RestockStatus]int64, error) {
	result := map[domain.RestockStatus]int64{
		domain.StatusCritical:         0,
		domain.StatusOverstock:        0,
		domain.StatusSafe:             0,
		domain.StatusInsufficientData: 0,
	}

	now := timeutil.Now()
	targetMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01")
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"target_month": targetMonth}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$status"},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
	cursor, err := r.db.Collection("forecast_results").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate forecast status summary: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Status domain.RestockStatus `bson:"_id"`
		Count  int64                `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("decode forecast status summary: %w", err)
	}
	for _, row := range rows {
		result[row.Status] = row.Count
	}
	return result, nil
}

func (r *ForecastRepo) LoadProductSales(ctx context.Context) ([]domain.ProductSalesData, error) {
	productCursor, err := r.db.Collection("products").Find(ctx, bson.M{"is_active": true})
	if err != nil {
		return nil, fmt.Errorf("load active products for forecast: %w", err)
	}
	defer productCursor.Close(ctx)

	var products []domain.Product
	if err := productCursor.All(ctx, &products); err != nil {
		return nil, fmt.Errorf("decode active products for forecast: %w", err)
	}
	if len(products) == 0 {
		return []domain.ProductSalesData{}, nil
	}

	productCodes := make([]string, 0, len(products))
	data := make([]domain.ProductSalesData, 0, len(products))
	indexByCode := make(map[string]int, len(products))
	for _, product := range products {
		indexByCode[product.Code] = len(data)
		productCodes = append(productCodes, product.Code)
		data = append(data, domain.ProductSalesData{
			Product: product,
			Sales:   make(map[string]int64),
		})
	}

	historyCursor, err := r.db.Collection("monthly_sales_history").Find(
		ctx,
		bson.M{"product_code": bson.M{"$in": productCodes}},
	)
	if err != nil {
		return nil, fmt.Errorf("load monthly sales history for forecast: %w", err)
	}
	defer historyCursor.Close(ctx)

	var histories []domain.MonthlySalesHistory
	if err := historyCursor.All(ctx, &histories); err != nil {
		return nil, fmt.Errorf("decode monthly sales history for forecast: %w", err)
	}
	for _, history := range histories {
		index, exists := indexByCode[history.ProductCode]
		if !exists || history.Year < 1 || history.Month < 1 || history.Month > 12 {
			continue
		}
		key := fmt.Sprintf("%04d-%02d", history.Year, history.Month)
		data[index].Sales[key] += history.QtySold
	}
	return data, nil
}

func forecastSort(field string) bson.D {
	switch field {
	case "predicted_demand":
		return bson.D{{Key: "predicted_demand", Value: -1}, {Key: "product_code", Value: 1}}
	case "current_stock":
		return bson.D{{Key: "current_stock", Value: -1}, {Key: "product_code", Value: 1}}
	case "product_code":
		return bson.D{{Key: "product_code", Value: 1}}
	case "restock_qty_asc":
		return bson.D{{Key: "restock_qty", Value: 1}, {Key: "product_code", Value: 1}}
	default:
		return bson.D{{Key: "restock_qty", Value: -1}, {Key: "product_code", Value: 1}}
	}
}
